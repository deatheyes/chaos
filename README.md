# chaos
A fault-injection scheduler

# Usage

    Usage of ./chaos:
    -a string
        http address to listen (default ":8677")
    -alsologtostderr
        log to standard error as well as files
    -c string
        config file, used only by controller currently (default "./conf/config.yaml")
    -g string
        grpc address to listen (default ":8678")
    -l string
        controller address, retrieved form env if not set, uesed by agent
    -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
    -log_dir string
        If non-empty, write log files in this directory
    -logtostderr
        log to standard error instead of files
    -r string
        run as [agent|controller] (default "agent")
    -s string
        selector, retrieved from env if not set, used by agent. eg: app=test,flag=aaaaa
    -stderrthreshold value
        logs at or above this threshold go to stderr
    -v value
        log level for V logs
    -version
        show version
    -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging

## Module
There is only one component which could run as two mode:
* agent
* controller

### Agent
Agent is deployed with the nodes of target cluster

#### Run

    /bin/chaos -l 127.0.0.1:8678 -g :8679 -logtostderr -r "agent" -s "role=follower,tag=myregion" -v 8

### Controller
Controler is the task scheduler which agents register to.

#### Config

    systemtap: # systemtap configuration
      script_dir: /root/fault-injection # systemtap script base dir
      probe_dir: /root/test_service/data # systemtap IO probe dir limitation
      pidfile: /root/test_service/pid # file to get pid
    service:
      A: # service unique name
        start: /root/test_service/start.sh # start command
        stop: /root/test_service/stop.sh # stop command

### Run

    ./bin/fault-injection  -logtostderr -r "controller" -v 8

### Stage
Stage setups a task DAG utilized to create controllable noise

#### Config

    name: s1 # unique name, detected by controller
    tasks: # task list
    - type: empty # type of task amount empty, service, ioerror, iodelay and partition
      name: t1 # unique name, detected by controller
      requires: # task dependences in DAG
      - "head"
    - type: partition
      name: t2
      arguments: # special configurations of task
        debug: "true"
      strategy: # node selector and task convertor
        selector: "role=follower" # define how to select a nodes
        ticker_cycle: 10 # task would be converted to a ticker if set
        ticker_last: 5   # task would be converted to a ticker if set
        timeout: 30      # the task would be converted to a timer if set
      requires:
      - "head"
    - type: service
      name: t3
      arguments:
        debug: "true"
      strategy:
        selector: "role=follower"
        ticker_cycle: 10
        ticker_last: 5
        timeout: 30
      requires:
      - "t1"
      - "t2"
    - type: service
      name: t4
      arguments:
        debug: "true"
      strategy:
        selector: "role=follower"
        ticker_cycle: 10
        ticker_last: 5
        timeout: 30
      requires:
      - "t3"

## API

### Run Stage
    
    curl -X POST http://127.0.0.1:8677/chaos/s1

### Stop Stage

    curl -X DELETE http://127.0.0.1:8677/chaos/s1

### Query Stage

    curl http://127.0.0.1:8677/chaos/{id:string}


* List{id:"list"}

list all stages as a string joined with ",", eg. "s1,s2,s3,s4"

* GetOne{id:$target}

get the content of the stages specified as a string

* GetAll{id:"all"}

get all content of the stages as a string joined with "----\n"