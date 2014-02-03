package main

import "log"
import "flag"
import "time"
import "strings"
import "net"
import "net/http"
import "io/ioutil"
import "strconv"
import "reflect"
import "encoding/json"
import "github.com/streadway/amqp"
import "github.com/marpaia/graphite-golang"

func rabbitmqConnect(uri string, queueName string) (queueConnection *amqp.Connection, queueChannel *amqp.Channel, err error) {
    queueConnection, err = amqp.Dial(uri)
    if err == nil {
        queueChannel, err = queueConnection.Channel();
    }
    return
}

func nonFatalError(msg string, err error, pauseMsec int) bool {
    if err == nil {
        return false
    }
    log.Printf("non-fatal error - %s: %s", msg, err)
    time.Sleep(time.Millisecond * time.Duration(pauseMsec))
    return true
}

func fetchUrl(requestUrl string) (body []byte, statusCode int, err error) {
    resp, err := http.Get(requestUrl)
    if err != nil {
        return
    }
    defer resp.Body.Close()
    statusCode = resp.StatusCode
    body, err = ioutil.ReadAll(resp.Body)
    return
}

func fetchQueueMetrics(mgmtUri string, prefix string) (metrics []graphite.Metric) {
    url := mgmtUri + "/api/queues?columns=name,message_stats.publish_details.rate"
    response, statusCode, err := fetchUrl(url)
    if err != nil || statusCode != 200 {
        log.Printf("error fetch rabbiqmq queues: %d - %s", statusCode, err)
        return
    }
    var stats []interface{}
    json.Unmarshal(response, &stats)
    for _, _stat := range stats {
        stat := _stat.(map[string]interface{})
        name := stat["name"].(string)
        if name == "" {
            continue
        }
        rate := 0.0
        if message_stats, ok := stat["message_stats"]; ok {
            if reflect.ValueOf(message_stats).Kind() == reflect.Map {
                if publish_details, ok := message_stats.(map[string]interface{})["publish_details"]; ok {
                    rate = publish_details.(map[string]interface{})["rate"].(float64)
                }
            }
        }
        metric := graphite.Metric{Name:prefix+"queue."+name,
            Value:strconv.Itoa(int(rate)),
            Timestamp:time.Now().Unix()}
        metrics = append(metrics, metric)
    }
    return
}

func fetchExchangeMetrics(mgmtUri string, prefix string) (metrics []graphite.Metric) {
    url := mgmtUri + "/api/exchanges?columns=name,message_stats_out.publish_details.rate"
    response, statusCode, err := fetchUrl(url)
    if err != nil || statusCode != 200 {
        log.Printf("error fetch rabbiqmq queues: %d - %s", statusCode, err)
        return
    }
    var stats []interface{}
    json.Unmarshal(response, &stats)
    for _, _stat := range stats {
        stat := _stat.(map[string]interface{})
        name := stat["name"].(string)
        if name == "" {
            continue
        }
        rate := 0.0
        if _, ok := stat["message_stats_out"]; ok {
            rate = stat["message_stats_out"].(map[string]interface{})["publish_details"].
                (map[string]interface{})["rate"].(float64)
        }
        metric := graphite.Metric{Name:prefix+"exchange."+name,
            Value:strconv.Itoa(int(rate)),
            Timestamp:time.Now().Unix()}
        metrics = append(metrics, metric)
    }
    return
}

func monitoring(uri string, queueName string, mgmtUri string, prefix string) {
    var (
        queueConn  *amqp.Connection
        queueChan  *amqp.Channel
        err error
    )
    for {
        log.Printf("fetch rabbitmq stats")
        var metrics []graphite.Metric
        for _, metric := range fetchQueueMetrics(mgmtUri, prefix) {
            metrics = append(metrics, metric)
        }
        for _, metric := range fetchExchangeMetrics(mgmtUri, prefix) {
            metrics = append(metrics, metric)
        }
        queueConn, queueChan, err = rabbitmqConnect(uri, queueName)
        if err != nil {
            time.Sleep(time.Second)
            continue
        }
        for _, metric := range metrics {
            body := []byte( metric.Name+"\t"+metric.Value+"\t"+strconv.FormatInt(metric.Timestamp, 10))
            msg := amqp.Publishing{ContentType:"text/plain",Body:body}
            queueChan.Publish("", queueName, false, false, msg)
        }
        queueChan.Close()
        queueConn.Close()
        time.Sleep(time.Second)
    }
}

func graphiteSendMetric(host string, port int, metric graphite.Metric) {
    graphiteConn, err := graphite.NewGraphite(host, port)
    if err != nil {
        log.Printf("skip metrics sending, because graphite not working: %s", err)
        return
    }
    graphiteConn.SendMetric(metric)
}


func metricListen(uri string, queueName string, graphiteHost string, graphitePort int) {
    queueConn, queueChan, err := rabbitmqConnect(uri, queueName)
    if nonFatalError("can't connect to rabbitmq", err, 5000) {
        return
    }
    defer queueConn.Close()
    defer queueChan.Close()
    queue, err := queueChan.QueueDeclare(queueName,true,false,false,false,nil)
    if nonFatalError("can't queue declare", err, 5000) {
        return
    }
    msgs, err := queueChan.Consume(queue.Name, "", true, false, false, false, nil)
    for msg := range msgs {
        data := strings.Split(string(msg.Body), "\t")
        timestamp, _ := strconv.ParseInt(data[2], 10, 64)
        metric := graphite.Metric{Name:data[0],Value:data[1],Timestamp:timestamp}
        //log.Printf("metric: %s = %s", data[0], data[1])
        graphiteSendMetric(graphiteHost, graphitePort, metric)
    }
}


func main() {
    log.Printf("Welcome to rabbitmq-graphite-tool")
    var (
        queue           string
        uri             string
        mgmtUri         string
        graphite        string
        prefix          string
    )

    flag.StringVar(&queue, 
        "rabbitmq-queue", "graphite", "incoming queue name for graphite metrics")
    flag.StringVar(&uri, 
        "rabbitmq-uri", "amqp://guest:guest@localhost:5672", "rabbitmq connection uri")
    flag.StringVar(&mgmtUri, 
        "rabbitmq-mgmt-uri", 
        "http://guest:guest@localhost:55672", "rabbitmq managment plugin address host:port")
    flag.StringVar(&graphite, 
        "graphite", "localhost:2003", "graphite server address host:port")
    flag.StringVar(&prefix, 
        "prefix", "rabbitmq.node01_", "prefix for rabbitmq monitoring in graphite")
    flag.Parse()

    log.Printf("rabbitmq-queue:     %s", queue)
    log.Printf("rabbitmq-uri:       %s", uri)
    log.Printf("rabbitmq-mgmt-uri:  %s", mgmtUri)
    log.Printf("graphite-addr:      %s", graphite)
    log.Printf("prefix:             %s", prefix)

    graphiteHost, _graphitePort, err := net.SplitHostPort(graphite)
    if err != nil {
        log.Fatalf("can't parse graphite host:port: %s", graphite)
        return
    }
    graphitePort, _ := strconv.Atoi(_graphitePort)

    go monitoring(uri, queue, mgmtUri, prefix)
    for {
        //<- make(chan bool)
        metricListen(uri, queue, graphiteHost, graphitePort)
    }
}