#!/usr/bin/bash

go run node.go 15000 127.0.0.1 2 >> logs.log 2>&1 &

sleep 0.5

for i in {1..20}
do
    # docker run -d -e port=$((15000+$i)) -e ip=10.1.0.$(($i+2)) -p $((15000+$i)):$((15000+$i)) --ip 10.1.0.$(($i+2)) --net mynetwork golang_node_praks1
    go run node.go $((15000+$i)) 127.0.0.1 2 >> logs.log 2>&1 &
    sleep 0.2
done

sleep 2

# ----

curl -d '{"fromToMap": {"John": {"name": "Mary","value": 5.75}}}' -X POST http://localhost:15001/transaction &

curl -d '{"fromToMap": {"John": {"name": "Jean","value": 3.75}}}' -X POST http://localhost:15001/transaction &

curl -d '{"fromToMap": {"Mary": {"name": "John","value": 10.75}}}' -X POST http://localhost:15001/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 7.75}}}' -X POST http://localhost:15001/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 6.69}}}' -X POST http://localhost:15001/transaction &

sleep 2

# ----

curl -d '{"fromToMap": {"John": {"name": "Mary","value": 5.75}}}' -X POST http://localhost:15006/transaction &

curl -d '{"fromToMap": {"John": {"name": "Jean","value": 3.75}}}' -X POST http://localhost:15007/transaction &

curl -d '{"fromToMap": {"Mary": {"name": "John","value": 10.75}}}' -X POST http://localhost:15008/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 7.75}}}' -X POST http://localhost:15009/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 6.69}}}' -X POST http://localhost:15010/transaction &

sleep 2

# ----

curl -d '{"fromToMap": {"John": {"name": "Mary","value": 5.75}}}' -X POST http://localhost:15011/transaction &

curl -d '{"fromToMap": {"John": {"name": "Jean","value": 3.75}}}' -X POST http://localhost:15012/transaction &

curl -d '{"fromToMap": {"Mary": {"name": "John","value": 10.75}}}' -X POST http://localhost:15013/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 7.75}}}' -X POST http://localhost:15014/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 6.69}}}' -X POST http://localhost:15015/transaction &

sleep 10

for i in {21..100}
do
    # docker run -d -e port=$((15000+$i)) -e ip=10.1.0.$(($i+2)) -p $((15000+$i)):$((15000+$i)) --ip 10.1.0.$(($i+2)) --net mynetwork golang_node_praks1
    go run node.go $((15000+$i)) 127.0.0.1 2 >> logs.log 2>&1 &
    sleep 0.2
done

curl -d '{"fromToMap": {"John": {"name": "Mary","value": 5.75}}}' -X POST http://localhost:15050/transaction &

curl -d '{"fromToMap": {"John": {"name": "Jean","value": 3.75}}}' -X POST http://localhost:15050/transaction &

curl -d '{"fromToMap": {"Mary": {"name": "John","value": 10.75}}}' -X POST http://localhost:15050/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 7.75}}}' -X POST http://localhost:15050/transaction &

curl -d '{"fromToMap": {"Jean": {"name": "Mary","value": 6.69}}}' -X POST http://localhost:15050/transaction &

sleep 2

# curl -d '' -X POST http://localhost:15006/block