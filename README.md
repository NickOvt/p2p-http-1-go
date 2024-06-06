# Simple P2P network and unsynchronized blockchain in Golang
> Author: Nikolai Ovtsinnikov 213084 IAIB @ 2024

#### Tech stack:
1. Golang
2. Golang standard libraries
   1. `crypto/sha256` - used to create the sha-256 hash
   2. `encoding/hex` - transforms raw sha-256 hash bytes to hex string
   2. `encoding/json` - used to encode/decode json
   3. `fmt` - stderr and stdout logging
   4. `io/ioutil` - reads file content
   5. `log` - file logging
   6. `net` - TCP/IP and other networking related parts
   7. `os` - communication with os. Creating, Reading files etc.
   8. `strconv` - string conversions. int -> str, float -> str, str -> int/float etc.
   9.  `strings` - string manipulation
   10. `sync` - goroutine syncing
   11. `time` - timestamp and date
> The project uses no external libraries, everything is done using the Golang provided standard libraries

#### Main points:
1. Project does not use built-in HTTP server and built-in HTTP client/server possibilities.  
The whole communication is done solely using TCP/IP socket server(s) and client(s) and HTTP  
Requests are parsed manually according to the RFC (or at least as close to possible to the RFC  
(considering I did this project alone and doing a complete HTTP server from scratch is no easy task  
especially if it needs to comply to the most up to date RFC))
2. Stemming from the previous point: The project massively utilises Go-s built-in concurrency (Goroutines)  
Every new connection to the server is a separate goroutine, every client connection is a separate goroutine.  
`HTTP/1.1` header `Connection: keep-alive` IS accounted and the TCP connection is kept alive if the headers is specified with  
that value, otherwise (as a HTTP server should) the request is processed, response is sent and the connection is closed.  
Other (important) HTTP headers are also accounted for `Content-Length`,`Content-Type`, `Connection`, this means that making   
the request to the nodes in a browser, via curl or Postman works correctly and does not result in a hanging connection or  
incorrect response.

#### Main P2P related points:
1. Transactions are held in mempool (i.e RAM) until a block is created, then all the transactions that are in the block  
are removed from mempool.
2. Each block is held in the root directory (yes I know I should've made a separate folder for this 🙄).  
Block filename format is `<block body sha-256 hash>` + `_` + `<block creation unix timestamp>` + `.txt`  
3. For the reasons of simplicity all nodes are connected to every other available node unless that other node  
disconnects from the network (goes offline). Main nodes (the nodes that are known to everyone) can be specified in the  
`main_nodes.json` configuration file. Every node tries to connect to each main node every 5 seconds, because main  
nodes are to be considered central nodes and if no central node is left then new nodes cannot connect to any other  
live nodes on the network, so central nodes are crucial for operation.

## Working principle
> Server:
1. Set up file logging
2. Load `main_nodes.json`
3. Get exec params `port` and `ip` if available. 
4. TCP server (hereafter just Server) starts on the machine on specified `port` and `ip`. Defaults are `8080` and `0.0.0.0` respectively.
5. Server loops through the `ip:port` strings in the `main_nodes.json` file and tries to form a connection with the given addresses.
6. Server executes the accept loop in a Goroutine (listens to connections)
7. If new connection add it to global pool of currently available connections and execute the read loop in a separate goroutine
8. Read loop reads in the `raw bytes` the server receives (if there is a read error such as an `EOF` data then the  
connection is closed and pools are cleaned (more about them later))
9. Server checks that the request is an HTTP request and processes it. Then checks if the HTTP requests is actually a `request` or  
perhaps a `response` and then acts accordingly either running `doResponse()` to process a received response or `doRequest()` to  
process the received request. Connection is kept alive if the header is specified otherwise the function returns which automatically  
closes the connection (other `nodes` receive `EOF` as well as any other caller)
10. Based on the `HTTP path`, `query params`, `path params`, `request type` processes the `request` or `response` as a usual HTTP server.

> Client:
1. When a server needs to connect to other nodes it opens a client connection to that node. Client connection has different local address  
compared to the node. Which means that in order to keep track of the actual node server addresses every inter-node HTTP request/response  
has the custom `X-Own-IP` header which holds the actual server ip of the node (not the client TCP connection IP which is different!).
2. Client works the same way as the server but without the accept loop as the client is an outgoing connection request. That means  
client also has a readloop that works completely identical to the server readloop and the same rules apply.

### API endpoints:
1. `GET` endpoints
   1. `/addr` - Retrieve the nodes this current node has connections to
   2. `/hello` - Test server connection
   3. `/getblocks/:blockhash` - Get all block hashes. If blockhash path param is given get blocks after the timestamp of the  
   given block (if it is found)
   4. `/getblockdata/:blockhash` - Get the block data corresponding to the given block hash
   5. `<no-route>` - Error 404
2. `POST` endpoints:
   1. `/transaction` - Create a transaction, add it to mempool, if this transaction does not exist yet (different hash) send it to all other nodes
   2. `/transactionReceive` - Node received a transaction from other node, check if has in mempool, if not add and spread,  
   otherwise just write back success and do nothing
   3. `/block` - Create a block with the transactions that the given node currently has. Check if block does not exist on filesystem,  
   if not write block to file, spread to other nodes. If exists write back success and do nothing
   4. `/blockReceive` - Node received a block from other nodes, check if already exists on filesystem, if exists write back success and do nothing,  
   if does not exist add to filesystem and spread to other nodes
   5. `<no-route>` Error 404

> @ 2024 by Nikolai Ovtsinnikov 😎👋