all:
	go build -o rabbitmq-graphite-tool rabbitmq-graphite-tool.go

depends:
	go get github.com/streadway/amqp
	go get github.com/marpaia/graphite-golang