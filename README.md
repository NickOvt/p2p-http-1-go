# Simple P2P network and synchronised blockchain in Golang
> Author: NickOvt @ 2024

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
   10. `sync` - goroutine syncing (Mutexes)
   11. `time` - timestamp and date
> The project uses no external libraries, everything is done using the Golang provided standard libraries (except for `mapstructure`)

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
2. Each block is kept in a file that has the following naming convention: `<ip of server><port of server>.txt`. All blocks of the corresponding node/server are held in that txt file in a special format.  
3. Every node is connected to `n` specified nodes (default to `2`). The value can be specified as an argument to the application. Main nodes (the nodes that are known to everyone) can be specified in the `main_nodes.json` config file. Every node tries to connect to each main node every `5` seconds, because main nodes are to be considered central nodes and if no central node is specified then new nodes cannot connect to other nodes (but as any node can be specified as a central node then you can always specify a known node that will allow you to connect to the rest of them). In any case knowing at least one node beforehand is crucial for connecting all nodes together.

## Working principle
> Server:
1. Set up file logging
2. Load `main_nodes.json`
3. Get exec params `port` and `ip` if available. `maxConnectionsArg` as third parameter if specified.
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
   3. `/getAllBlocks` - Get all blocks data as json list of `Block` objects.
   3. `/getblocks/:blockhash` - Get all block hashes. If blockhash path param is given get blocks after the timestamp of the  
   given block (if it is found)
   4. `/getblockdata/:blockhash` - Get the block data corresponding to the given block hash
   5. `<no-route>` - Error 404
2. `POST` endpoints:
   1. `/transaction` - Create a transaction, add it to mempool, if this transaction does not exist yet (different hash) send it to all other nodes. After `5` have been received create a `Block`, send it out, clear used 5 transactions from mempool.
   2. `/blockReceive` - Node received a block from other nodes, check if already exists on filesystem, if exists write back success and do nothing,  
   if does not exist add to filesystem and spread to other nodes
   3. `<no-route>` Error 404

> @ 2024 by NickOvt 😎👋  
> This if my first Golang project
>
> As of 14.10.2024 there is a slight issue in blockchain syncing and it is not synced properly. Will be fixed soon