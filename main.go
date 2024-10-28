package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Transaction struct {
	FromToMap map[string]*PersonReceiver `json:"fromToMap"` // who send how much to whom when
	Hash      string                     `json:"hash"`
}

type PersonReceiver struct {
	Name      string    `json:"name"`
	Value     float32   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type Message struct {
	Success bool        `json:"success"`
	ErrMsg  string      `json:"errMsg"`
	Msg     interface{} `json:"msg"`
	MsgType string      `json:"msgType"`
}

type Node struct {
	ip     string
	port   string
	active bool
}

type InitialNodes struct {
	Nodes []string `json:"nodes"`
}

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
}

func doRequest(conn net.Conn, requestData []string, requestPayload string, requestType string, headers map[string]string) {
	pathParams := getPathParams(requestData[0])

	// on every HTTP request get the server IP of the caller node (if it is a node, which is specified by the X-Own-IP header)
	mux.Lock()
	xOwnIpVal, isOwnIpOk := headers["X-Own-IP"]
	if isOwnIpOk {
		_, doesAddrExistAlready := existingNodesAddresses[xOwnIpVal]

		// check if node already exists
		if !doesAddrExistAlready {
			// new node, check if can add to pool
			if currentConnections < maxConnections {
				currentConnections++
				callerServerIpSplitted := strings.Split(xOwnIpVal, ":")
				newNode := Node{ip: callerServerIpSplitted[0], port: callerServerIpSplitted[1], active: true}
				existingNodesAddresses[xOwnIpVal] = &newNode
				existingClientToNodeMap[conn.RemoteAddr().String()] = xOwnIpVal
				existingNodeToClientMap[xOwnIpVal] = conn.RemoteAddr().String()

				log.Printf("Added new node from the given HTTP request: client (%s) node (%s) \n", conn.RemoteAddr().String(), xOwnIpVal)
			} else {
				log.Println("TOO MANY CONNECTIONS ALREADY DO NOT ADD NODE (Request)")
				log.Printf("Node tried to connect from the given HTTP request: client (%s) node (%s) \n", conn.RemoteAddr().String(), xOwnIpVal)
				// can't accept any more persistent connections, just return known nodes
				nodesToReturn := InitialNodes{}

				var nodes []*Node

				for _, val := range existingNodesAddresses {
					nodes = append(nodes, val)
				}

				for _, node := range nodes {
					nodeString := node.ip + ":" + node.port
					nodesToReturn.Nodes = append(nodesToReturn.Nodes, nodeString)
				}

				res := HTTPResponse{}

				messageBack := Message{Success: true, ErrMsg: "", Msg: nodesToReturn, MsgType: "addr"}

				headers["Connection"] = "close"

				res.setStatus(OK200)
				res.setData(messageBack)
				res.setHeader("Content-Type", CONTENT_JSON)
				res.setHeader("X-Own-IP", serverAddress)
				res.setCtxHeaders(headers)

				conn.Write(res.buildBytes())
				mux.Unlock()
				return
			}
		}
	}
	mux.Unlock()

	switch requestType {
	case "GET":
		if pathParams[0] == "addr" {
			// return all known nodes of this node
			nodesToReturn := InitialNodes{}

			var nodes []*Node

			mux.RLock()
			for _, val := range existingNodesAddresses {
				nodes = append(nodes, val)
			}
			mux.RUnlock()

			for _, node := range nodes {
				nodeString := node.ip + ":" + node.port
				nodesToReturn.Nodes = append(nodesToReturn.Nodes, nodeString)
			}

			res := HTTPResponse{}

			messageBack := Message{Success: true, ErrMsg: "", Msg: nodesToReturn, MsgType: "addr"}

			res.setStatus(OK200)
			res.setData(messageBack)
			res.setHeader("Content-Type", CONTENT_JSON)
			res.setHeader("X-Own-IP", serverAddress)
			res.setCtxHeaders(headers)

			conn.Write(res.buildBytes())
		} else if pathParams[0] == "getAllBlocks" {
			allBlocks := getBlockDataOnDisk()

			messageBack := Message{Success: true, ErrMsg: "", Msg: allBlocks, MsgType: "getAllBlocks"}

			res := HTTPResponse{}
			res.setData(messageBack)
			res.setStatus(OK200)
			res.setCtxHeaders(headers)
			res.setHeader("Content-Type", CONTENT_JSON)

			conn.Write(res.buildBytes())
		} else if pathParams[0] == "hello" {
			messageBack := Message{Success: true, ErrMsg: "", Msg: "Hello There!", MsgType: "hello"}

			res := HTTPResponse{}
			res.setData(messageBack)
			res.setStatus(OK200)
			res.setCtxHeaders(headers)
			res.setHeader("Content-Type", CONTENT_JSON)

			conn.Write(res.buildBytes())
		} else if pathParams[0] == "getblocks" {
			givenBlockHash := ""
			hasBlockHashGiven := false

			if len(pathParams) > 1 {
				givenBlockHash = pathParams[1]
				hasBlockHashGiven = true
			}

			if len(givenBlockHash) != 64 && hasBlockHashGiven {
				messageBack := Message{Success: false, ErrMsg: "Block hash incorrect length. Expected 64-byte sha-256 hash as hex string", Msg: "", MsgType: "getBlocksData"}

				res := HTTPResponse{}
				res.setData(messageBack)
				res.setStatus(ERROR500)
				res.setCtxHeaders(headers)
				res.setHeader("Content-Type", CONTENT_JSON)

				conn.Write(res.buildBytes())
				return
			}

			// return the list of hashes of all blocks

			blockHashes := []string{}

			blocksOnDisk := getBlocksHashTimestampOnDisk()

			// find the block by hash

			foundBlock := false
			var foundBlockTimestamp time.Time

			if hasBlockHashGiven { // given the hash, find the block
				for _, blockData := range blocksOnDisk {
					blockHash := blockData[0]

					if blockHash == givenBlockHash {
						// found block
						foundBlock = true
						foundBlockTimestamp, _ = time.Parse(time.RFC3339, blockData[1])
					}
				}
			}

			if !foundBlock && hasBlockHashGiven {
				// hash was given but no block found return error
				res := HTTPResponse{}

				messageBack := Message{Success: false, ErrMsg: "Did not find block corresponding to hash", Msg: "", MsgType: "getBlocks"}

				res.setData(messageBack)
				res.setStatus(ERROR404)
				res.setCtxHeaders(headers)
				res.setHeader("Content-Type", CONTENT_JSON)
				conn.Write(res.buildBytes())
				return
			}

			for _, blockData := range blocksOnDisk {
				blockHash := blockData[0]
				blockTimestamp, _ := time.Parse(time.RFC3339, blockData[1])

				if foundBlock && blockTimestamp.Unix() >= foundBlockTimestamp.Unix() {
					// check for timestamp

					// the given block is after the provided unixtimestamp param, add it to res
					blockHashes = append(blockHashes, blockHash)
				} else if !foundBlock && !hasBlockHashGiven {
					blockHashes = append(blockHashes, blockHash)
				}
			}

			messageBack := Message{Success: true, ErrMsg: "", Msg: blockHashes, MsgType: "getBlocks"}

			res := HTTPResponse{}
			res.setData(messageBack)
			res.setStatus(OK200)
			res.setCtxHeaders(headers)
			res.setHeader("Content-Type", CONTENT_JSON)

			conn.Write(res.buildBytes())
		} else if pathParams[0] == "getblockdata" {
			if len(pathParams) < 2 {
				messageBack := Message{Success: false, ErrMsg: "Block hash not given", Msg: "", MsgType: "getBlocksData"}

				res := HTTPResponse{}
				res.setData(messageBack)
				res.setStatus(ERROR500)
				res.setCtxHeaders(headers)
				res.setHeader("Content-Type", CONTENT_JSON)

				conn.Write(res.buildBytes())
				return
			}

			givenBlockHash := pathParams[1]

			if len(givenBlockHash) != 64 {
				messageBack := Message{Success: false, ErrMsg: "Block hash incorrect length. Expected 64-byte sha-256 hash as hex string", Msg: "", MsgType: "getBlocksData"}

				res := HTTPResponse{status: ERROR500, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader("Content-Type", CONTENT_JSON)

				conn.Write(res.buildBytes())
				return
			}

			blocksOnDisk := getBlockDataOnDisk()

			for _, blockOnDisk := range blocksOnDisk {
				if blockOnDisk.Hash == givenBlockHash {
					// found the block
					messageBack := Message{Success: true, ErrMsg: "", Msg: blockOnDisk, MsgType: "getBlocksData"}

					res := HTTPResponse{status: OK200, ctxHeaders: headers}
					res.setData(messageBack)
					res.setHeader("Content-Type", CONTENT_JSON)

					conn.Write(res.buildBytes())
					return
				}
			}

			messageBack := Message{Success: false, ErrMsg: "No block found", Msg: "", MsgType: "getBlocksData"}

			res := HTTPResponse{status: ERROR404, ctxHeaders: headers}
			res.setData(messageBack)
			res.setHeader("Content-Type", CONTENT_JSON)

			conn.Write(res.buildBytes())

		} else {
			messageBack := Message{Success: false, ErrMsg: "Did not find specified path!", Msg: "", MsgType: "no-route"}

			res := HTTPResponse{status: ERROR404, ctxHeaders: headers}
			res.setData(messageBack)
			res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

			conn.Write(res.buildBytes())
		}
	case "POST":
		if pathParams[0] == "transaction" {
			// Get transaction data and send it to other active nodes, API endpoint for outside use
			// by default assume JSON
			var transaction Transaction

			err := json.Unmarshal([]byte(requestPayload), &transaction)

			receivedFromNode := isOwnIpOk

			if err != nil {
				log.Printf("Error unmarshaling transaction: (%s)", err)

				messageBack := Message{Success: false, ErrMsg: "Transaction receive error. Incorrect transaction format.", Msg: "", MsgType: "transaction"}

				res := HTTPResponse{status: ERROR500, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

				conn.Write(res.buildBytes())
				return // conn write error and the break
			}

			if !receivedFromNode {
				// got a transaction from outside connection (not node)

				// add current timestamp to all receivers
				currentTimestamp := time.Now()
				for _, val := range transaction.FromToMap {
					// if !val.Timestamp.IsZero() {
					val.Timestamp = currentTimestamp
					// }
				}

				// add trsansaction hash

				transactionBodyJson, err := json.Marshal(transaction.FromToMap)

				if err != nil {
					log.Println(err)

					messageBack := Message{Success: false, ErrMsg: "Internal server transaction error!", Msg: "", MsgType: "transaction"}

					res := HTTPResponse{status: ERROR500, ctxHeaders: headers}
					res.setData(messageBack)
					res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

					conn.Write(res.buildBytes())

					return
				}

				transaction.Hash = sha256encode(transactionBodyJson)
			}

			createdBlock := false

			// add transaction to mempool

			// check if transaction exists in mempool
			mux.RLock()
			_, alreadyExistsThisTransaction := transactions[transaction.Hash]
			mux.RUnlock()

			if !alreadyExistsThisTransaction {
				// no such transaction, add it to mempool

				// check that transaction is not present in any known blocks as well

				allCurrentBlocks := getBlockDataOnDisk()

				for _, block := range allCurrentBlocks {
					if alreadyExistsThisTransaction {
						break
					}

					for _, inBlockTransaction := range block.Transactions {
						if transaction.Hash == inBlockTransaction.Hash {
							// this transaction already exists in a known block
							alreadyExistsThisTransaction = true
							break
						}
					}
				}

				if !alreadyExistsThisTransaction {
					mux.Lock()
					transactions[transaction.Hash] = &transaction
					mux.Unlock()
				}
			}

			log.Println(existingNodeToClientMap)
			log.Println(existingClientsAddresses)

			// check that after adding current transaction there is 5 transactions already
			if len(transactions) == 5 {
				createBlock(conn, headers, isOwnIpOk, xOwnIpVal)
				createdBlock = true
			}

			// send transaction to all other connected nodes
			if !alreadyExistsThisTransaction && !createdBlock {
				mux.Lock()
				for _, val := range existingNodesAddresses {
					// get connection for the node
					existingClientRemoteAddr, existingClientRemoteAddrExists := existingNodeToClientMap[val.ip+":"+val.port]

					if !existingClientRemoteAddrExists {
						continue // did not find client remote address for given connected node (why?)
					}

					if val.ip+":"+val.port == xOwnIpVal {
						// do not send block in circular
						continue
					}

					existingClient, existingClientExists := existingClientsAddresses[existingClientRemoteAddr]

					if !existingClientExists {
						continue // did not find client object for given connected node (why though??)
					}

					transactionToSend, _ := json.Marshal(transaction) // I doubt there will be an error here :D

					log.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

					req := HTTPRequest{requestType: REQ_POST, path: "/transaction", version: VERSION1_1, data: string(transactionToSend)}
					req.setHeader("X-Own-IP", serverAddress)

					existingClient.conn.Write(req.buildBytes())
				}
				mux.Unlock()
			}

			if alreadyExistsThisTransaction {
				// send back response to initial caller, transaction already exists in mempool or blocks
				messageBack := Message{Success: true, Msg: "Accepted transaction. This transaction already exists. Discarded transaction. Hash:" + transaction.Hash, ErrMsg: "", MsgType: "transaction"}

				res := HTTPResponse{status: OK200, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

				conn.Write(res.buildBytes())
				return
			}

			// send back response to initial caller, new transaction, all good
			messageBack := Message{Success: true, ErrMsg: "", Msg: "Accepted transaction, delivered to known nodes. Thank You!. Hash:" + transaction.Hash, MsgType: "transaction"}

			res := HTTPResponse{status: OK200, ctxHeaders: headers}
			res.setData(messageBack)
			res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

			conn.Write(res.buildBytes())
			return
		} else if pathParams[0] == "blockReceive" {
			// else if pathParams[0] == "block" {
			// 	// get current transactions, put into a block, sent it to other, others remove transactions from mempool as well and add block to their blockchain
			// 	createBlock(conn, headers)
			// }
			var receivedBlock Block

			err := json.Unmarshal([]byte(requestPayload), &receivedBlock)

			if err != nil {
				log.Printf("Error unmarshaling block: (%s) (%s)|||", err, requestPayload)

				messageBack := Message{Success: false, ErrMsg: "Internal server block error!", Msg: "", MsgType: "blockReceive"}

				res := HTTPResponse{status: ERROR500, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

				conn.Write(res.buildBytes())

				return
			}

			ok := false

			blocksOnDisk := getBlockDataOnDisk()
			lastBlockBiggestNr := 0
			// lastBlockBiggestNrHash := ""

			for _, blockData := range blocksOnDisk {
				if blockData.Hash == receivedBlock.Hash {
					// already got block with given hash, do not add it
					ok = true
				}

				if blockData.Nr > lastBlockBiggestNr {
					lastBlockBiggestNr = blockData.Nr
					// lastBlockBiggestNrHash = blockData.Hash
				}
			}

			mux.Lock()
			for _, transaction := range receivedBlock.Transactions {
				delete(transactions, transaction.Hash)
			}
			mux.Unlock()

			if lastBlockBiggestNr+1 < receivedBlock.Nr {
				// received block has higher nr that currently available (more than 1 higher)
				// need to fetch all blocks from the connection
				// do not continue adding
				// send back response that we got the block but we're behind

				messageBack := Message{Success: false, ErrMsg: "", Msg: "Block received. But we're behind in blocks. Will sync", MsgType: "blockReceive"}

				res := HTTPResponse{status: OK200, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

				conn.Write(res.buildBytes())

				conn.Write([]byte("GET /getAllBlocks HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:" + serverAddress + "\r\n\r\n"))
				return
			} else if receivedBlock.Nr <= lastBlockBiggestNr {
				// received block either has the same nr or is lower!
				// that means we got a block from a node that is behind, discard the block, send back response
				messageBack := Message{Success: false, ErrMsg: "", Msg: "Block received. But you're behind, will discard this block. Sync", MsgType: "blockReceive"}

				res := HTTPResponse{status: OK200, ctxHeaders: headers}
				res.setData(messageBack)
				res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

				conn.Write(res.buildBytes())
				return
			}

			if !ok {
				// such block does not exist yet so add it to files, send to other
				// write block to file
				// f, err := os.Create(receivedBlock.Hash + "_" + strconv.FormatInt(receivedBlock.Timestamp.Unix(), 10) + ".txt")
				f, err := os.OpenFile(serverAddressHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

				blockBodyJson, _ := json.Marshal(receivedBlock.Transactions)

				_, err1 := f.Write([]byte(blockToString(receivedBlock, blockBodyJson)))

				if err != nil || err1 != nil {
					log.Println(err, err1)
				}

				f.Close()

				mux.RLock()
				// send block to others
				for _, val := range existingNodesAddresses {
					// get connection for the node
					existingClientRemoteAddr, existingClientRemoteAddrExists := existingNodeToClientMap[val.ip+":"+val.port]

					if !existingClientRemoteAddrExists {
						continue
					}

					if existingClientRemoteAddr == xOwnIpVal {
						// do not send block in circular
						continue
					}

					existingClient, existingClientExists := existingClientsAddresses[existingClientRemoteAddr]

					if !existingClientExists {
						continue
					}

					// jsonMarshal block and send it over
					blockToSend, _ := json.Marshal(receivedBlock)

					log.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

					req := HTTPRequest{requestType: REQ_POST, path: "/blockReceive", version: VERSION1_1, data: string(blockToSend)}
					req.setHeader("X-Own-IP", serverAddress)
					existingClient.conn.Write(req.buildBytes())
				}
				mux.RUnlock()
			}

			messageBack := Message{Success: true, ErrMsg: "", Msg: "Block received. Thank You! Hash:" + receivedBlock.Hash, MsgType: "blockReceive"}

			res := HTTPResponse{status: OK200, ctxHeaders: headers}
			res.setData(messageBack)
			res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

			conn.Write(res.buildBytes())
		} else {
			messageBack := Message{Success: false, ErrMsg: "No such POST route!", Msg: "", MsgType: "no-route"}

			res := HTTPResponse{status: ERROR404, ctxHeaders: headers}
			res.setData(messageBack)
			res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

			conn.Write(res.buildBytes())
		}
	default:
		messageBack := Message{Success: false, ErrMsg: "Server does not support this HTTP request type", Msg: "", MsgType: "no-route"}

		res := HTTPResponse{status: ERROR500, ctxHeaders: headers}
		res.setData(messageBack)
		res.setHeader(HDR_CONTENT_TYPE, CONTENT_JSON)

		conn.Write(res.buildBytes())
	}
}

func doResponse(conn net.Conn, msgData []string, msgPayload string, headers map[string]string, connType string) {
	log.Println("================================")
	log.Println(connType + " read (response):")
	log.Println("================================")
	log.Println(msgData, msgPayload)
	log.Println("================================")

	mux.Lock()
	val, ok := headers["X-Own-IP"]
	connectionHeaderVal := headers["Connection"] // inter-node calls always have the header
	if ok {
		_, doesAddrExistAlready := existingNodesAddresses[val]

		// add node only if this node doesn't already exists in known nodes AND the connection is keep-alive, otherwise there is no point in adding the node it will be removed anyway after
		if !doesAddrExistAlready && strings.ToLower(connectionHeaderVal) != "close" && currentConnections < maxConnections {
			currentConnections++
			callerServerIpSplitted := strings.Split(val, ":")
			newNode := Node{ip: callerServerIpSplitted[0], port: callerServerIpSplitted[1], active: true}
			existingNodesAddresses[val] = &newNode
			existingClientToNodeMap[conn.RemoteAddr().String()] = val
			existingNodeToClientMap[val] = conn.RemoteAddr().String()

			log.Printf("Added new node from the given HTTP response: client (%s) node (%s) \n", conn.RemoteAddr().String(), val)
		}
	}
	mux.Unlock()

	var message Message

	err := json.Unmarshal([]byte(msgPayload), &message)

	if err != nil {
		log.Println("Error Unmarshaling message, check data type, probably got response from some 3rd party (non-node)")
	}

	switch message.MsgType {
	case "addr":
		var gotNodes InitialNodes

		err := mapToObj(message.Msg, &gotNodes)

		if err != nil {
			log.Printf("There was an error receiving nodes!\n")
			return
		}

		log.Println(gotNodes)
		log.Println(currentConnections)
		log.Println("GOT ADDRS")

		maxConnectionsVariable := -1

		mux.Lock()

		i := currentConnections

		for _, gotNode := range gotNodes.Nodes {
			if i < maxConnections+maxConnectionsVariable {
				nodeAddr := gotNode
				_, hasTriedThisNode := triedNodes[nodeAddr]

				if hasTriedThisNode {
					// already tried to connect to this node, do not try again, try others
					continue
				}

				_, ok := existingNodesAddresses[nodeAddr]

				if !ok && nodeAddr != serverAddress {
					// such node does not exist yet, add it, connect to it
					client := connectToClient(nodeAddr)

					if client == nil {
						continue
					}

					i++
					triedNodes[nodeAddr] = true

					// Request neighbors from other nodes
					existingClientsAddresses[client.remoteAddr] = client

					req := HTTPRequest{requestType: REQ_GET, path: "/addr", version: VERSION1_1}
					req.setHeader("X-Own-IP", serverAddress)

					client.conn.Write(req.buildBytes()) // nodes as JSON string {"nodes": []}
				}
			}
		}
		mux.Unlock()
	case "transaction":
		// got response from sending/receiving transaction

		mux.RLock()
		nodeAddr := existingClientToNodeMap[conn.RemoteAddr().String()]
		mux.RUnlock()

		if message.Success {
			_, ok := message.Msg.(string)

			if ok {
				log.Printf("This node: (%s), received transaction response from client (%s) which corresponds to node (%s). Transaction receive was successful.\n", serverAddress, conn.RemoteAddr().String(), nodeAddr)
			} else {
				log.Printf("There was an error receiving transaction in this node, other node had error\n")
			}
		} else {
			log.Printf("There was an error receiving transaction in this node, other node had error\n")
		}
		break
	case "blockReceive":
		// check that blockReceive is success: false and if we're behind to appropriate request
		if !message.Success {
			// failed
			// check if we're behind
			msgString, ok := message.Msg.(string)

			if ok {
				msgSplitted := strings.Split(msgString, ".")
				if strings.TrimSpace(msgSplitted[len(msgSplitted)-1]) == "Sync" {
					// sync
					conn.Write([]byte("GET /getAllBlocks HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:" + serverAddress + "\r\n\r\n"))
				}
			} else {
				log.Println("There was an error receiving a block!")
			}
		}
	case "getAllBlocks":
		if message.Success {
			var receivedBlocks []Block

			receivedBlocksAny := message.Msg.([]interface{})
			for _, receivedBlockAny := range receivedBlocksAny {
				var block Block
				err := mapToObj(receivedBlockAny, &block)

				if err != nil {
					log.Println("There was an error receiving all blocks!")
					return
				}
				receivedBlocks = append(receivedBlocks, block)
			}

			f, err := os.OpenFile(serverAddressHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

			for _, block := range receivedBlocks {
				blockBodyJson, _ := json.Marshal(block.Transactions)
				_, err1 := f.Write([]byte(blockToString(block, blockBodyJson)))

				if err1 != nil {
					log.Println(err1)
				}
			}

			if err != nil {
				log.Println(err)
			}

			f.Close()
		}
	}

	log.Println("================================")
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp4", s.listenAddr)

	if err != nil {
		return err
	}

	log.Printf("Server started on %s\r\n", ln.Addr().String())
	serverAddress = ln.Addr().String()

	serverAddressHash = strings.ReplaceAll(strings.ReplaceAll(serverAddress, ".", ""), ":", "")

	f, _ := os.OpenFile(serverAddressHash+".txt", os.O_CREATE|os.O_APPEND, 0666)
	f.Close()

	// Connect to initial nodes and get the nodes they have
	connectToInitialNodes()

	ticker := time.NewTicker(5000 * time.Millisecond)
	ticker2 := time.NewTicker(5 * time.Minute)

	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				log.Println(existingNodesAddresses)
				log.Println(currentConnections)
				connectToInitialNodes()
			case <-ticker2.C:
				triedNodes = make(map[string]bool)
			}
		}
	}() // async IIFE

	defer ln.Close()
	s.ln = ln

	go s.acceptLoop()

	<-s.quitch

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Println("Accept error", err)
			continue
		}

		log.Println("new connection to the server", conn.RemoteAddr())
		client := Client{remoteAddr: conn.RemoteAddr().String(), conn: conn}

		mux.Lock()
		existingClientsAddresses[client.remoteAddr] = &client
		mux.Unlock()

		go readLoop(conn, "SERVER")
	}
}

func readLoop(conn net.Conn, connType string) {
	defer conn.Close()
	buf := make([]byte, 2048*10)

	for {
		n, err := conn.Read(buf) // overwrites buffer

		if err != nil {
			log.Println("read error", err)

			if err.Error() == "EOF" || errors.Is(err, syscall.WSAECONNRESET) {
				// check remote addr
				log.Print(conn.RemoteAddr().String())

				mux.RLock()
				val, ok := existingClientToNodeMap[conn.RemoteAddr().String()]
				mux.RUnlock()

				if ok {
					// there is a node corresponding to the closed client connection, delete the connection
					_, doesNodeExist := existingNodesAddresses[val]
					mux.Lock()
					defer mux.Unlock()

					if doesNodeExist {
						delete(existingNodesAddresses, val)
					}
					delete(existingClientToNodeMap, conn.RemoteAddr().String())
					delete(existingNodeToClientMap, val)
					delete(existingClientsAddresses, conn.RemoteAddr().String()) // remove the client connection from map
					currentConnections--
					log.Printf("Deleted client obj %s", conn.RemoteAddr().String())
				}

				return
			}

			continue
		}

		log.Println("_______________________________________")
		log.Println(connType + " read:")
		log.Println("---------------------------------------")

		// msg := buf[:n]
		msg := buf[:n]
		msgString := string(msg)
		log.Println(msgString)
		log.Println("---------------------------------------")

		// HTTP request/response
		if strings.Contains(msgString, "HTTP") {
			for msgString != "" {
				currentHTTPStringsHeaders := strings.Split(msgString, "\r\n\r\n")[0]

				currentHttpStringsHeadersSplitted := strings.Split(currentHTTPStringsHeaders, "\r\n")

				// check if request or response, if request get request type and proceed with doRequest
				// if a response then proceed with response
				requestType := strings.Split(currentHttpStringsHeadersSplitted[0], " ")[0]

				var msgPayload []string

				headers := parseHeadersString(currentHttpStringsHeadersSplitted[1:])

				requestContainsHttp := strings.Contains(requestType, "HTTP")
				contentLength, contentLengthOk := headers["Content-Length"]

				var contentLengthVal int
				if contentLengthOk {
					contentLengthVal, _ = strconv.Atoi(contentLength)
				}

				if !contentLengthOk && (requestType == "POST" || requestType == "PUT") {
					// no content length for POST or PUT request
					conn.Write([]byte("HTTP request with types POST or PUT are not allowed without content-length header!"))
					conn.Close()
				}

				if contentLengthOk {
					// have content length
					// split
					data := strings.Split(msgString, "\r\n\r\n")[1][:contentLengthVal]

					leftOver := strings.Split(msgString, data)[1]
					msgString = leftOver
					msgPayload = strings.Split(data, "\r\n")
				} else {
					msgString = "" // empty the msg string so that we do not continue after the loop
				}

				if requestContainsHttp {
					// request is actually a response, do response
					doResponse(conn, currentHttpStringsHeadersSplitted, strings.Join(msgPayload, ""), headers, connType)
				} else {
					doRequest(conn, currentHttpStringsHeadersSplitted, strings.Join(msgPayload, ""), requestType, headers)
				}

				connectionType, ok := headers["Connection"]

				if !ok {
					// if no connection type header default to close
					connectionType = "close"
				}

				if strings.ToLower(connectionType) == "keep-alive" {
					// if keep alive then continue
					continue
				}

				// else close connection then cleanup
				log.Print(conn.RemoteAddr().String())

				mux.RLock()
				val, ok := existingClientToNodeMap[conn.RemoteAddr().String()]
				mux.RUnlock()

				if ok {
					// there is a node corresponding to the closed client connection, delete the connection
					_, doesNodeExist := existingNodesAddresses[val]
					mux.Lock()
					defer mux.Unlock()

					if doesNodeExist {
						delete(existingNodesAddresses, val)
					}

					delete(existingClientToNodeMap, conn.RemoteAddr().String())
					delete(existingNodeToClientMap, val)
					delete(existingClientsAddresses, conn.RemoteAddr().String()) // remove the client connection from map
					currentConnections--
					log.Printf("Deleted client obj %s", conn.RemoteAddr().String())
				}
				return
			}
		}
	}
}

// TODO: Implement or use BiMap instead
var existingNodesAddresses = map[string]*Node{}     // node ip -> node object
var existingClientToNodeMap = map[string]string{}   // client ip -> node ip if the client was a distributed node
var existingNodeToClientMap = map[string]string{}   // node ip -> client ip if the client was a distributed node
var transactions = map[string]*Transaction{}        // transaction hash -> transaction data
var existingClientsAddresses = map[string]*Client{} // client ip -> client object
var serverAddress string                            // server ip and port
var serverAddressHash string                        // hash of server address
var maxConnections int                              // max allowed connections
var currentConnections = 0                          // count of currently active connections

var triedNodes = map[string]bool{} // map of node ip -> bool. Used when connecting to new nodes

var mux sync.RWMutex // syncing mutex

var initialNodesAdresses = []string{} // save initial nodes to RAM so to save file reads (it is not expected to hold large amounts of initial nodes)

func main() {
	mainNodesJsonFileByteValue, mainNodesFileErr := os.ReadFile("main_nodes.json")

	// Load initial nodes
	if mainNodesFileErr != nil {
		log.Fatalf("Error opening JSON config file with hardcoded nodes! %v", mainNodesFileErr)
	}

	var initialNodes InitialNodes
	json.Unmarshal(mainNodesJsonFileByteValue, &initialNodes)

	// Parse loaded initial nodes
	for _, neighbourNodeString := range initialNodes.Nodes {
		splitted := strings.Split(neighbourNodeString, ":")

		initialNodesAdresses = append(initialNodesAdresses, splitted[0]+":"+splitted[1])
	}

	portPtr := flag.String("port", "8080", "Port value")
	ipPtr := flag.String("ip", "127.0.0.1", "IP value")
	maxConnectionsPtr := flag.Int("maxconns", 2, "Number of maximum allowed connections")
	logFilePath := flag.String("logfile", "", "Log file path")

	flag.Parse()

	port := *portPtr
	ip := *ipPtr
	maxConnections = *maxConnectionsPtr

	if *logFilePath != "" {
		f, err := os.OpenFile(*logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

		if err != nil {
			log.Fatalf("Error opening log file!: %v", err)
		}
		defer f.Close()

		log.SetOutput(f)
		log.SetFlags(log.Ldate | log.Ltime)
	}

	server := NewServer(ip + ":" + port)

	log.Fatal(server.Start())
}
