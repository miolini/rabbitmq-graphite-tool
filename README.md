rabbitmq-graphite-tool
======================

Send metrics to Graphite over RabbitMQ with internal msg rates. Writen on Go language.

With this tool you can do next:

- send yours metrics to Graphite over RabbitMQ queues
- send RabbitMQ internal rates in msg/s to Graphite

## Install

1. git clone https://github.com/miolini/rabbitmq-graphite-tool.git
2. make depends
3. make

## Usage

```
./rabbitmq-graphite-tool --help

Usage of ./rabbitmq-graphite-tool:
  -graphite="localhost:2003": graphite server address host:port
  -prefix="rabbitmq.node01.": prefix for rabbitmq monitoring in graphite
  -rabbitmq-mgmt-uri="http://guest:guest@localhost:55672": rabbitmq managment plugin address host:port
  -rabbitmq-queue="graphite": incoming queue name for graphite metrics
  -rabbitmq-uri="amqp://guest:guest@localhost:5672": rabbitmq connection uri
```

## Result

![Image](graphite.png?raw=true)


Enjoy!


Artem Andreenko,
Openstat.
