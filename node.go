package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	OK200    = "200 OK"
	ERROR500 = "500 Internal Server Error"
	ERROR404 = "404 Not Found"
	ERROR400 = "400 Bad Request"
	ERROR401 = "401 Unauthorized"
	ERROR403 = "403 Forbidden"
	ERROR411 = "411 Length Required"
)

const (
	CONTENT_PLAIN = "text/plain"
	CONTENT_HTML  = "text/html"
	CONTENT_JSON  = "application/json"
)

type Client struct {
	remoteAddr string
	conn       net.Conn
}

type Transaction struct {
	FromToMap map[string]*PersonReceiver `json:"fromToMap"` // who send how much to whom when
	Hash      string                     `json:"hash"`
}

type Block struct {
	Hash         string        `json:"hash"`
	Transactions []Transaction `json:"transactions"`
	Timestamp    time.Time     `json:"timestamp"`
	Nr           int           `json:"nr"`
	MerkleRoot   string        `json:"merkle_root"`
	Count        int           `json:"transaction_count"`
	Nonce        string        `json:"nonce"`
	PrevHash     string        `json:"prev_hash"`
}

type PersonReceiver struct {
	Name      string    `json:"name"`
	Value     float32   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type Message struct {
	Success bool   `json:"success"`
	ErrMsg  string `json:"errMsg"`
	Msg     string `json:"msg"`
	MsgType string `json:"msgType"`
}

// Server  >
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

func sha256encode(val []byte) string {
	h := sha256.New()

	h.Write([]byte(val))

	bs := h.Sum(nil)

	return hex.EncodeToString(bs)
}

func getMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 1 {
		return transactions[0].Hash
	}

	list := []Transaction{}
	len := len(transactions)

	i := 0
	for i < len {
		currentTransaction := transactions[i]
		if i+1 >= len {
			value := currentTransaction.Hash + currentTransaction.Hash

			list = append(list, Transaction{Hash: sha256encode([]byte(value))})
			break
		}

		nextTransaction := transactions[i+1]
		value := currentTransaction.Hash + nextTransaction.Hash

		list = append(list, Transaction{Hash: sha256encode([]byte(value))})

		i += 2
	}

	return getMerkleRoot(list)
}

func createBlock(conn net.Conn, headers map[string]string, isNode bool) {
	// get current transactions, put into a block, sent it to other, others remove transactions from mempool as well and add block to their blockchain
	var currentTransactions []Transaction

	mux.RLock()
	for _, val := range transactions {
		currentTransactions = append(currentTransactions, *val)
	}
	mux.RUnlock()

	var block Block

	block.Timestamp = time.Now()

	block.Transactions = currentTransactions

	block.Count = len(currentTransactions)

	// remove all added transactions
	mux.Lock()
	for key := range transactions {
		delete(transactions, key)
	}
	mux.Unlock()

	blocksOnDisk := getBlockDataOnDisk()

	existingBlocksHashes := map[string]bool{}

	// find last block to get prev_hash as well as prev_nr to base the new nr on
	maxLastPrev := 0
	// maxLastPrevBlockIndex := 0
	maxLastPrevBlockHash := ""

	for _, blockData := range blocksOnDisk {
		existingBlocksHashes[blockData.Hash] = true

		if blockData.Nr > maxLastPrev {
			maxLastPrev = blockData.Nr
			// maxLastPrevBlockIndex = idx
			maxLastPrevBlockHash = blockData.Hash
		}
	}

	block.Nr = maxLastPrev + 1
	block.PrevHash = maxLastPrevBlockHash

	// calculate merkle root hash of transactions
	block.MerkleRoot = getMerkleRoot(block.Transactions)

	blockBodyJson, _ := json.Marshal(block.Transactions)

	wholeBlockJson, _ := json.Marshal(block)

	block.Hash = sha256encode(wholeBlockJson) // replace with finding nonce and hash through Proof of Work (PoW)

	// check if a block with current hash already exists on filesystem
	_, doesBlockAlreadyExist := existingBlocksHashes[block.Hash]

	if !isNode {
		for _, blockData := range blocksOnDisk {
			if blockData.Hash == block.Hash {
				// already got block with given hash, do not add it
				messageBack := Message{Success: false, ErrMsg: "Already got a block with identical data, aborted", Msg: "", MsgType: "block"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

				conn.Write(dataToWriteBack)

				return
			}
		}

		if len(block.Hash) == 0 {
			messageBack := Message{Success: false, ErrMsg: "Empty block hash abort", Msg: "", MsgType: "block"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)

			return
		}
	}

	// write block to file
	// f, err := os.WriteFile(block.Hash + "_" + strconv.FormatInt(block.Timestamp.Unix(), 10) + ".txt")

	if !doesBlockAlreadyExist {
		f, err := os.OpenFile(serverAddresHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

		_, err1 := f.Write([]byte(blockToString(block, blockBodyJson)))

		if err != nil || err1 != nil {
			fmt.Println(err, err1)
		}

		f.Close()

		// send block to others
		mux.Lock()
		for _, val := range existingNodesAddresses {
			// get connection for the node
			existingClientRemoteAddr, ok := existingNodeToClientMap[val.ip+":"+val.port]

			fmt.Println(existingClientRemoteAddr)

			if existingClientRemoteAddr == conn.RemoteAddr().String() {
				// do not send transaction in circular
				continue
			}

			if !ok {
				// no client, connect if possible and set node to active
			} else {
				// there is existing client retrieve it

				existingClient, ok := existingClientsAddresses[existingClientRemoteAddr]

				if !ok {
					// no client exists, connect if possible and set node to active
				} else {
					// client exists do request
					// jsonMarshal transaction and send it over
					blockToSend, _ := json.Marshal(block)

					fmt.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

					doClientRequest(existingClient.conn, "POST /blockReceive HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n"+string(blockToSend))
				}
			}
		}
		mux.Unlock()
	}

	if !isNode {
		messageBack := Message{Success: true, ErrMsg: "", Msg: "Made block and send it out", MsgType: "block"}

		messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

		dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

		conn.Write(dataToWriteBack)

		return
	}
}

func parseHeaders(headers []string) map[string]string {
	// Parse headers into a map

	headersMap := make(map[string]string)

	for _, header := range headers {
		splittedHeader := strings.SplitN(strings.ReplaceAll(header, " ", ""), ":", 2)
		headerKey := splittedHeader[0]
		headerValue := splittedHeader[1]

		headersMap[headerKey] = headerValue
	}

	return headersMap
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

				fmt.Printf("Added new node from the given HTTP request: client (%s) node (%s) \n", conn.RemoteAddr().String(), xOwnIpVal)
			} else {
				fmt.Println("TOO MANY CONNECTIONS ALREADY DO NOT ADD NODE (Request)")
				fmt.Printf("Node tried to connect from the given HTTP request: client (%s) node (%s) \n", conn.RemoteAddr().String(), xOwnIpVal)
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

				nodesToReturnJson, err := json.Marshal(nodesToReturn)

				var dataToWriteBack []byte

				messageBack := Message{Success: true, ErrMsg: "", Msg: string(nodesToReturnJson), MsgType: "addr"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				headers["Connection"] = "close"

				if err != nil {
					messageBack := Message{Success: false, ErrMsg: "Error JSON encoding nodes", Msg: "", MsgType: "addr"}

					messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

					dataToWriteBack = dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON, "X-Own-IP": serverAddres}, headers)
				} else {
					dataToWriteBack = dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON, "X-Own-IP": serverAddres}, headers)
				}

				conn.Write(dataToWriteBack)
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

			nodesToReturnJson, err := json.Marshal(nodesToReturn)

			var dataToWriteBack []byte

			messageBack := Message{Success: true, ErrMsg: "", Msg: string(nodesToReturnJson), MsgType: "addr"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			if err != nil {
				messageBack := Message{Success: false, ErrMsg: "Error JSON encoding nodes", Msg: "", MsgType: "addr"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				dataToWriteBack = dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON, "X-Own-IP": serverAddres}, headers)
			} else {
				dataToWriteBack = dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON, "X-Own-IP": serverAddres}, headers)
			}

			conn.Write(dataToWriteBack)
		} else if pathParams[0] == "addrSearch" {
			// implement
		} else if pathParams[0] == "getAllBlocks" {
			allBlocks := getBlockDataOnDisk()

			allBlocksJson, _ := json.Marshal(allBlocks)

			messageBack := Message{Success: true, ErrMsg: "", Msg: string(allBlocksJson), MsgType: "getAllBlocks"}
			messageBackJsonBytes, _ := json.Marshal(messageBack)
			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)
			conn.Write(dataToWriteBack)
		} else if pathParams[0] == "hello" {
			messageBack := Message{Success: true, ErrMsg: "", Msg: "Hello There!", MsgType: "hello"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)
		} else if pathParams[0] == "getblocks" {
			givenBlockHash := ""
			hasBlockHashGiven := false

			if len(pathParams) > 1 {
				givenBlockHash = pathParams[1]
				hasBlockHashGiven = true
			}

			if len(givenBlockHash) != 64 && hasBlockHashGiven {
				messageBack := Message{Success: false, ErrMsg: "Block hash incorrect length. Expected 64-byte sha-256 hash as hex string", Msg: "", MsgType: "getBlocksData"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

				conn.Write(dataToWriteBack)
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
				messageBack := Message{Success: false, ErrMsg: "Did not find block corresponding to hash", Msg: "", MsgType: "getBlocks"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances
				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR404, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)
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

			blockHashesJson, _ := json.Marshal(blockHashes)

			messageBack := Message{Success: true, ErrMsg: "", Msg: string(blockHashesJson), MsgType: "getBlocks"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)
		} else if pathParams[0] == "getblockdata" {
			if len(pathParams) < 2 {
				messageBack := Message{Success: false, ErrMsg: "Block hash not given", Msg: "", MsgType: "getBlocksData"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

				conn.Write(dataToWriteBack)
				return
			}

			givenBlockHash := pathParams[1]

			if len(givenBlockHash) != 64 {
				messageBack := Message{Success: false, ErrMsg: "Block hash incorrect length. Expected 64-byte sha-256 hash as hex string", Msg: "", MsgType: "getBlocksData"}

				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

				conn.Write(dataToWriteBack)
				return
			}

			blocksOnDisk := getBlockDataOnDisk()

			for _, blockOnDisk := range blocksOnDisk {
				if blockOnDisk.Hash == givenBlockHash {
					// found the block

					blockAsJson, _ := json.Marshal(blockOnDisk)

					messageBack := Message{Success: true, ErrMsg: "", Msg: string(blockAsJson), MsgType: "getBlocksData"}

					messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

					dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

					conn.Write(dataToWriteBack)
					return
				}
			}

			messageBack := Message{Success: false, ErrMsg: "No block found", Msg: "", MsgType: "getBlocksData"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR404, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)

		} else {
			messageBack := Message{Success: false, ErrMsg: "Did not find specified path!", Msg: "", MsgType: "no-route"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR404, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)
		}
		break
	case "POST":
		if pathParams[0] == "transaction" {
			// Get transaction data and send it to other active nodes, API endpoint for outside use
			// by default assume JSON
			var transaction Transaction

			err := json.Unmarshal([]byte(requestPayload), &transaction)

			if err != nil {
				fmt.Println(err)

				messageBack := Message{Success: false, ErrMsg: "Incorrect transaction format, check", Msg: "", MsgType: "transaction"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)

				return // conn write error and the break
			}

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
				fmt.Println(err)

				messageBack := Message{Success: false, ErrMsg: "Internal server transaction error!", Msg: "", MsgType: "transaction"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)

				return
			}

			transaction.Hash = sha256encode(transactionBodyJson)

			createdBlock := false

			// add transaction to mempool

			mux.RLock()
			_, ok := transactions[transaction.Hash]
			mux.RUnlock()

			alreadyExistsThisTransaction := false

			if !ok {
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

			fmt.Println(existingNodeToClientMap)
			fmt.Println(existingClientsAddresses)

			// check that after adding current transaction there is 5 transactions already
			if len(transactions) == 5 {
				createBlock(conn, headers, isOwnIpOk)
				createdBlock = true
			}

			// send transaction to all other connected nodes
			if !alreadyExistsThisTransaction && !createdBlock {
				mux.Lock()
				for _, val := range existingNodesAddresses {
					// get connection for the node
					existingClientRemoteAddr, ok := existingNodeToClientMap[val.ip+":"+val.port]

					fmt.Println(existingClientRemoteAddr)

					if !ok {
						// no client, connect if possible and set node to active
					} else {
						// there is existing client retrieve it

						existingClient, ok := existingClientsAddresses[existingClientRemoteAddr]

						if !ok {
							// no client exists, connect if possible and set node to active
						} else {
							// client exists do request
							// jsonMarshal transaction and send it over
							transactionToSend, _ := json.Marshal(transaction) // I doubt there will be an error here :D

							fmt.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

							doClientRequest(existingClient.conn, "POST /transactionReceive HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n"+string(transactionToSend))
						}
					}
				}
				mux.Unlock()

				fmt.Println("Transaction send back data to initial conn")
			}

			if alreadyExistsThisTransaction {
				// send back response to initial caller, transaction already exists in mempool or blocks
				messageBack := Message{Success: false, ErrMsg: "Accepted transaction. This transaction already exists. Discarded transaction", Msg: "", MsgType: "transaction"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)
				return
			}

			// send back response to initial caller, new transaction, all good
			messageBack := Message{Success: true, ErrMsg: "", Msg: "Accepted transaction, delivered to known nodes", MsgType: "transaction"}
			messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)
			conn.Write(dataToWriteBack)
			return
		} else if pathParams[0] == "transactionReceive" {
			var receivedTransaction Transaction

			err := json.Unmarshal([]byte(requestPayload), &receivedTransaction)

			if err != nil {
				fmt.Printf("Error unmarshaling transaction: (%s)", err)

				messageBack := Message{Success: false, ErrMsg: "Transaction receive error", Msg: "", MsgType: "transactionReceive"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)
				return
			}

			createdBlock := false

			// check if transaction exists in mempool
			mux.RLock()
			_, ok := transactions[receivedTransaction.Hash]
			mux.RUnlock()

			alreadyExistsThisTransaction := false

			if !ok {
				// such transaction does not exist in the mempool yet,
				// check if transaction exists in any of the blocks
				allCurrentBlocks := getBlockDataOnDisk()

				for _, block := range allCurrentBlocks {
					if alreadyExistsThisTransaction {
						break
					}
					for _, inBlockTransaction := range block.Transactions {
						if receivedTransaction.Hash == inBlockTransaction.Hash {
							// this transaction already exists in a known block
							alreadyExistsThisTransaction = true
							break
						}
					}
				}

				if !alreadyExistsThisTransaction {
					// new transaction, add to mempool, send to others

					mux.Lock()
					transactions[receivedTransaction.Hash] = &receivedTransaction
					mux.Unlock()

					// check if now there is 5 transactions, if yes, create block and send out
					if len(transactions) == 5 {
						createBlock(conn, headers, isOwnIpOk)
						createdBlock = true
					}

					if !createdBlock {
						mux.Lock()

						// send transaction to other nodes
						for _, val := range existingNodesAddresses {
							// get connection for the node
							existingClientRemoteAddr, ok := existingNodeToClientMap[val.ip+":"+val.port]

							fmt.Println(existingClientRemoteAddr)

							if existingClientRemoteAddr == conn.RemoteAddr().String() {
								// do not send transaction in circular
								continue
							}

							if !ok {
								// no client, connect if possible and set node to active
							} else {
								// there is existing client retrieve it

								existingClient, ok := existingClientsAddresses[existingClientRemoteAddr]

								if !ok {
									// no client exists, connect if possible and set node to active
								} else {
									// client exists do request
									// jsonMarshal transaction and send it over
									transactionToSend, _ := json.Marshal(receivedTransaction) // I doubt there will be an error here :D

									fmt.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

									doClientRequest(existingClient.conn, "POST /transactionReceive HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n"+string(transactionToSend))
								}
							}
						}
						mux.Unlock()
					}
				}
			}

			messageBack := Message{Success: true, ErrMsg: "", Msg: "Transaction received. Thank You! Hash:" + receivedTransaction.Hash, MsgType: "transactionReceive"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)

			// receive transaction and send it to other nodes (transaction is received from a node and not the API), so check that there is an X-Own-IP header
		} else if pathParams[0] == "blockReceive" {
			// else if pathParams[0] == "block" {
			// 	// get current transactions, put into a block, sent it to other, others remove transactions from mempool as well and add block to their blockchain
			// 	createBlock(conn, headers)
			// }
			var receivedBlock Block

			err := json.Unmarshal([]byte(requestPayload), &receivedBlock)

			if err != nil {
				fmt.Printf("Error unmarshaling block: (%s) (%s)|||", err, requestPayload)

				messageBack := Message{Success: false, ErrMsg: "Internal server block error!", Msg: "", MsgType: "blockReceive"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // pretty sure it won't fail unless some magical thing happens

				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)

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
				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances
				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)
				doClientRequest(conn, "GET /getAllBlocks HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n")
				return
			} else if receivedBlock.Nr <= lastBlockBiggestNr {
				// received block either has the same nr or is lower!
				// that means we got a block from a node that is behind, discard the block, send back response
				messageBack := Message{Success: false, ErrMsg: "", Msg: "Block received. But you're behind, will discard this block. Sync", MsgType: "blockReceive"}
				messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances
				dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)
				conn.Write(dataToWriteBack)
				return
			}

			if !ok {
				// such block does not exist yet so add it to files, send to other
				// write block to file
				// f, err := os.Create(receivedBlock.Hash + "_" + strconv.FormatInt(receivedBlock.Timestamp.Unix(), 10) + ".txt")
				f, err := os.OpenFile(serverAddresHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

				blockBodyJson, _ := json.Marshal(receivedBlock.Transactions)

				_, err1 := f.Write([]byte(blockToString(receivedBlock, blockBodyJson)))

				if err != nil || err1 != nil {
					fmt.Println(err, err1)
				}

				f.Close()

				mux.RLock()
				// send block to others
				for _, val := range existingNodesAddresses {
					// get connection for the node
					existingClientRemoteAddr, ok := existingNodeToClientMap[val.ip+":"+val.port]

					fmt.Println(existingClientRemoteAddr)

					if existingClientRemoteAddr == conn.RemoteAddr().String() {
						// do not send block in circular
						continue
					}

					if !ok {
						// no client, connect if possible and set node to active
					} else {
						// there is existing client retrieve it

						existingClient, ok := existingClientsAddresses[existingClientRemoteAddr]

						if !ok {
							// no client exists, connect if possible and set node to active
						} else {
							// client exists do request
							// jsonMarshal block and send it over
							blockToSend, _ := json.Marshal(receivedBlock)

							fmt.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

							doClientRequest(existingClient.conn, "POST /blockReceive HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n"+string(blockToSend))
						}
					}
				}
				mux.RUnlock()
			}

			messageBack := Message{Success: true, ErrMsg: "", Msg: "Block received. Thank You! Hash:" + receivedBlock.Hash, MsgType: "blockReceive"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), OK200, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)
		} else {
			messageBack := Message{Success: false, ErrMsg: "No such POST route!", Msg: "", MsgType: "no-route"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR404, map[string]string{"Content-Type": CONTENT_JSON}, headers)

			conn.Write(dataToWriteBack)
		}
		break
	default:
		messageBack := Message{Success: false, ErrMsg: "Server does not support this HTTP request type", Msg: "", MsgType: "no-route"}

		messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

		dataToWriteBack := dataToWrite(string(messageBackJsonBytes), ERROR500, map[string]string{"Content-Type": CONTENT_JSON}, headers)

		conn.Write(dataToWriteBack)
		break
	}
}

// Return the hash + timestamp (as iso string) of blocks on disk
func getBlocksHashTimestampOnDisk() [][]string {
	ledgerFileData, err := os.ReadFile(serverAddresHash + ".txt")

	if err != nil {
		log.Fatal(err)
	}

	ledgerFileDataStringsplitted := strings.Split(string(ledgerFileData), "\r\n")

	blockData := [][]string{}

	ledgerFileDataStringsplitted = ledgerFileDataStringsplitted[:len(ledgerFileDataStringsplitted)-1]

	for _, e := range ledgerFileDataStringsplitted {
		// only block files are .txt files in this P2P network
		// this is a block, get hash from filenames
		splitted := strings.Split(e, "\n")

		blockTimestamp := splitted[1]

		blockHash := splitted[0]

		blockData = append(blockData, []string{blockHash, blockTimestamp})
	}

	return blockData
}

func getBlockDataOnDisk() []*Block {
	ledgerFileData, err := os.ReadFile(serverAddresHash + ".txt")

	if err != nil {
		log.Fatal(err)
	}

	ledgerFileDataStringsplitted := strings.Split(string(ledgerFileData), "\r\n")

	ledgerFileDataStringsplitted = ledgerFileDataStringsplitted[:len(ledgerFileDataStringsplitted)-1]

	var blocks []*Block

	for _, e := range ledgerFileDataStringsplitted {
		blocks = append(blocks, blockStringToBlock(e))
	}

	return blocks
}

func blockToString(block Block, blockBodyJson []byte) string {
	blockString := ""

	blockString += block.Hash + "\n"
	blockString += block.Timestamp.Format(time.RFC3339) + "\n"
	blockString += "--- raw json start\n"
	blockString += string(blockBodyJson) + "\n"
	blockString += "--- raw json end\n"
	blockString += ">>> transactions:\n"

	for _, transaction := range block.Transactions {
		for key, val := range transaction.FromToMap {
			floatVal := fmt.Sprintf("%f", val.Value)
			blockString += key + " sent " + floatVal + " to " + val.Name + " at " + val.Timestamp.Format(time.RFC3339) + "\n"
		}
	}

	blockString += ">>>\n"
	blockString += "count: " + strconv.Itoa(block.Count) + "\n"
	blockString += "nr: " + strconv.Itoa(block.Nr) + "\n"
	blockString += "prev_hash: " + block.PrevHash + "\n"
	blockString += "merkle_root: " + block.MerkleRoot + "\n"
	blockString += "nonce: " + block.Nonce + "\n"
	blockString += "<<<\r\n"

	return blockString
}

func blockStringToBlock(blockString string) *Block {
	blockStringSplitted := strings.Split(blockString, "\n")

	var block Block

	block.Hash = blockStringSplitted[0]

	time, err := time.Parse(time.RFC3339, blockStringSplitted[1])

	if err != nil {
		return nil
	}

	block.Timestamp = time

	transactionsJsonString := blockStringSplitted[3]
	err2 := json.Unmarshal([]byte(transactionsJsonString), &block.Transactions)

	if err2 != nil {
		return nil
	}

	block.Nr, _ = strconv.Atoi(strings.TrimSpace(strings.Split(blockStringSplitted[len(blockStringSplitted)-5], ":")[1]))
	block.Count, _ = strconv.Atoi(strings.TrimSpace(strings.Split(blockStringSplitted[len(blockStringSplitted)-6], ":")[1]))
	block.PrevHash = strings.TrimSpace(strings.Split(blockStringSplitted[len(blockStringSplitted)-4], ":")[1])
	block.MerkleRoot = strings.TrimSpace(strings.Split(blockStringSplitted[len(blockStringSplitted)-3], ":")[1])
	block.Nonce = strings.TrimSpace(strings.Split(blockStringSplitted[len(blockStringSplitted)-2], ":")[1])

	return &block
}

func doResponse(conn net.Conn, msgData []string, msgPayload string, headers map[string]string, connType string) {
	fmt.Println("================================")
	fmt.Println(connType + " read (response):")
	fmt.Println("================================")
	fmt.Println(msgData, msgPayload)
	fmt.Println("================================")

	mux.Lock()
	val, ok := headers["X-Own-IP"]
	connectionHeaderVal, _ := headers["Connection"] // inter-node calls always have the header
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

			fmt.Printf("Added new node from the given HTTP response: client (%s) node (%s) \n", conn.RemoteAddr().String(), val)
		}
	}
	mux.Unlock()

	var message Message

	err := json.Unmarshal([]byte(msgPayload), &message)

	if err != nil {
		fmt.Println("Error Unmarshaling message, check data type, probably got response from some 3rd party (non-node)")
	}

	switch message.MsgType {
	case "addr":
		var gotNodes InitialNodes
		json.Unmarshal([]byte(message.Msg), &gotNodes)

		fmt.Println(gotNodes)
		fmt.Println(currentConnections)
		fmt.Println("GOT ADDRS")

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

				if !ok && nodeAddr != serverAddres {
					// such node does not exist yet, add it, connect to it
					client := connectToClient(nodeAddr)

					if client == nil {
						continue
					}

					i++
					triedNodes[nodeAddr] = true

					// Request neighbors from other nodes
					existingClientsAddresses[client.remoteAddr] = client
					// existingNodeToClientMap[nodeAddr] = client.remoteAddr
					// existingClientToNodeMap[client.remoteAddr] = nodeAddr

					// existingNodesAddresses[nodeAddr] = &Node{ip: splitted[0], port: splitted[1], active: true}

					// fmt.Println(client.conn.LocalAddr().String(), client.conn.RemoteAddr().String())

					doClientRequest(client.conn, "GET /addr HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n") // nodes as JSON string {"nodes": []}
				}
			}
		}
		mux.Unlock()
		break
	case "transaction":
		// got response from sending transaction
		break
	case "transactionReceive":
		mux.RLock()

		nodeAddr, _ := existingClientToNodeMap[conn.RemoteAddr().String()]
		mux.RUnlock()

		if message.Success {
			transactionHash := strings.Split(message.Msg, ":")[1]

			mux.RLock()
			_, ok := transactions[transactionHash]
			mux.RUnlock()

			var receivedString string

			if ok {
				receivedString = "has already received"
			} else {
				receivedString = "first time received"
			}

			fmt.Printf("This node: (%s), received transaction from client (%s) which corresponds to node (%s). This node %s this transaction\n", serverAddres, conn.RemoteAddr().String(), nodeAddr, receivedString)
			break
		} else {
			fmt.Printf("There was an error receiving transaction in this node, other node had error\n")
		}
	case "blockReceive":
		// check that blockReceive is success: false and if we're behind to appropriate request
		if !message.Success {
			// failed
			// check if we're behind
			msgSplitted := strings.Split(message.Msg, ".")
			if strings.TrimSpace(msgSplitted[len(msgSplitted)-1]) == "Sync" {
				// sync
				doClientRequest(conn, "GET /getAllBlocks HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n")
			}
		}
	case "getAllBlocks":
		if message.Success {
			var receivedBlocks []Block

			json.Unmarshal([]byte(message.Msg), &receivedBlocks)

			f, err := os.OpenFile(serverAddresHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

			for _, block := range receivedBlocks {
				blockBodyJson, _ := json.Marshal(block.Transactions)
				_, err1 := f.Write([]byte(blockToString(block, blockBodyJson)))

				if err1 != nil {
					fmt.Println(err1)
				}
			}

			if err != nil {
				fmt.Println(err)
			}

			f.Close()
		}
	}

	fmt.Println("================================")
}

func getPathParams(httpString string) []string {
	pathParams := strings.Split(strings.Split(httpString, " ")[1], "/")
	return pathParams[1:] // first elem is empty string
}

func getPathParamsValues(pathParams []string, paramsWithValues []string) map[string]string {
	pathParamsWithValues := map[string]string{}

	paramsWithValuesMap := map[string]int{}

	for index, paramWithValue := range paramsWithValues {
		paramsWithValuesMap[paramWithValue] = index
	}

	for index, pathParam := range pathParams {
		_, ok := paramsWithValuesMap[pathParam]
		if !ok {
			// There no such path param with value
			continue
		}

		// next val in the list of pathParams is the actual value of the parameter
		pathParamVal := pathParams[index+1]

		pathParamsWithValues[pathParam] = pathParamVal
	}

	return pathParamsWithValues
}

func getQueryParams(httpString string) map[string]string {
	splittedHttpString := strings.Split(httpString, " ")
	notparsedQueryParams := strings.Split(splittedHttpString[1], "?")

	if len(notparsedQueryParams) < 2 {
		// If there is no ? then length will be == 1
		return map[string]string{}
	}

	queryParams := map[string]string{}

	notparsedQueryParams2 := strings.Split(notparsedQueryParams[1], "&")

	for _, valString := range notparsedQueryParams2 {
		valStringSplitted := strings.Split(valString, "=")
		key := valStringSplitted[0]
		val := valStringSplitted[1]

		queryParams[key] = val
	}

	// there are query params available

	return queryParams
}

func dataToWrite(data string, status string, headers map[string]string, requestHeaders map[string]string) []byte {
	dataToWriteBack := "HTTP/1.1 "
	dataToWriteBack += status + "\r\n"

	for key, val := range headers {
		dataToWriteBack += key + ":" + val + "\r\n"
	}

	contentLength := len(data)

	dataToWriteBack += "Content-Length:" + strconv.Itoa(contentLength) + "\r\n"

	connectionType, ok := requestHeaders["Connection"]

	if ok {
		if strings.ToLower(connectionType) != "keep-alive" {
			dataToWriteBack += "Connection: close\r\n"
		} else {
			dataToWriteBack += "Connection: keep-alive\r\n"
		}
	}

	dataToWriteBack += "\r\n"

	dataToWriteBack += data

	return []byte(dataToWriteBack)
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

	fmt.Printf("Server started on %s\r\n", ln.Addr().String())
	serverAddres = ln.Addr().String()

	serverAddresHash = strings.ReplaceAll(strings.ReplaceAll(serverAddres, ".", ""), ":", "")

	f, err := os.OpenFile(serverAddresHash+".txt", os.O_CREATE|os.O_APPEND, 0666)
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
				fmt.Println(existingNodesAddresses)
				fmt.Println(currentConnections)
				// connectToInitialNodes()
			case <-ticker2.C:
				triedNodes = make(map[string]bool)
			}
		}
	}()

	defer ln.Close()
	s.ln = ln

	go s.acceptLoop()

	<-s.quitch

	return nil
}

func connectToInitialNodes() {
	// Connect to initial nodes and get the nodes they have
	i := 0
	mux.Lock()
	// connect to maxConnections - 1 connections (so that there is always one more connection to add from outside)
	for i < maxConnections-1 && i < len(initialNodesAdresses) {
		mainNodeAddr := initialNodesAdresses[i]

		i++

		if mainNodeAddr == serverAddres {
			continue
		}

		_, ok := existingNodesAddresses[mainNodeAddr]

		if ok {
			// already have connection for this main node
			continue
		}

		client := connectToClient(mainNodeAddr)

		if client == nil {
			continue
		}

		// Request neighbors from other nodes
		existingClientsAddresses[client.remoteAddr] = client
		// existingNodeToClientMap[mainNodeAddr] = client.remoteAddr
		// existingClientToNodeMap[client.remoteAddr] = mainNodeAddr

		// splitted := strings.Split(mainNodeAddr, ":")
		// triedNodes[mainNodeAddr] = true

		// existingNodesAddresses[mainNodeAddr] = &Node{ip: splitted[0], port: splitted[1], active: true}

		fmt.Println(client.conn.LocalAddr().String(), client.conn.RemoteAddr().String())

		doClientRequest(client.conn, "GET /addr HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:"+serverAddres+"\r\n\r\n") // nodes as JSON string {"nodes": []}
	}
	mux.Unlock()
}

// get payload from data (request)
func getPayload(data string) string {
	msgData := strings.Split(data, "\r\n")

	var msgPayload []string
	for index, headerString := range msgData[1:] {
		if headerString != "" {
			continue
		} else {
			msgPayload = msgData[index+1:]
			break // if line is empty then this is highly likely the data portion line breaker
		}
	}

	return strings.Join(msgPayload, "")
}

// write to client conn
func doClientRequest(clientConn net.Conn, data string) {
	dataSplitted := strings.Split(data, "\r\n\r\n")

	if strings.Contains(data, "POST") {
		// there is some content
		contentLength := len(dataSplitted[1])

		contentLengthStr := "\r\nContent-Length:" + strconv.Itoa(contentLength)
		dataSplitted[0] += contentLengthStr
	}

	dataSplitted[0] += "\r\n\r\n"

	data = strings.Join(dataSplitted, "")

	clientConn.Write([]byte(data))
}

func connectToClient(addr string) *Client {
	clientConn, err := net.Dial("tcp", addr)

	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("Connected to client: %s\n", clientConn.RemoteAddr().String())

	go readLoop(clientConn, "CLIENT")

	return &Client{remoteAddr: clientConn.RemoteAddr().String(), conn: clientConn}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			fmt.Println("Accept error", err)
			continue
		}

		fmt.Println("new connection to the server", conn.RemoteAddr())
		client := Client{remoteAddr: conn.RemoteAddr().String(), conn: conn}

		mux.Lock()
		existingClientsAddresses[client.remoteAddr] = &client
		mux.Unlock()

		// connected remote might be anything, perhaps it is not a node. So do not add it as a node

		// remoteAddrSplitted := strings.Split(conn.RemoteAddr().String(), ":")
		// newNode := Node{ip: remoteAddrSplitted[0], port: remoteAddrSplitted[1], active: true}
		// nodes = append(nodes, newNode)
		// existingNodesAddresses[conn.RemoteAddr().String()] = newNode

		go readLoop(conn, "SERVER")
	}
}

func readLoop(conn net.Conn, connType string) {
	defer conn.Close()
	buf := make([]byte, 2048*10)

	for {
		n, err := conn.Read(buf) // overwrites buffer

		if err != nil {
			fmt.Println("read error", err)

			if err.Error() == "EOF" {
				// check remote addr
				fmt.Print(conn.RemoteAddr().String())

				mux.RLock()
				val, ok := existingClientToNodeMap[conn.RemoteAddr().String()]
				mux.RUnlock()

				if ok {
					// there is a node corresponding to the closed client connection, delete the connection
					_, doesNodeExist := existingNodesAddresses[val]
					mux.Lock()
					if doesNodeExist {
						delete(existingNodesAddresses, val)
					}

					delete(existingClientToNodeMap, conn.RemoteAddr().String())
					delete(existingNodeToClientMap, val)
					delete(existingClientsAddresses, conn.RemoteAddr().String()) // remove the client connection from map
					currentConnections--
					mux.Unlock()
					fmt.Printf("Deleted client obj %s", conn.RemoteAddr().String())
				}

				return
			}

			continue
		}

		fmt.Println("_______________________________________")
		fmt.Println(connType + " read:")
		fmt.Println("---------------------------------------")

		// msg := buf[:n]
		msg := buf[:n]
		msgString := string(msg)
		fmt.Println(msgString)
		fmt.Println("---------------------------------------")

		// HTTP request/response
		if strings.Contains(msgString, "HTTP") {
			for msgString != "" {
				currentHTTPStringsHeaders := strings.Split(msgString, "\r\n\r\n")[0]

				currentHttpStringsHeadersSplitted := strings.Split(currentHTTPStringsHeaders, "\r\n")

				// check if request or response, if request get request type and proceed with doRequest
				// if a response then proceed with response
				requestType := strings.Split(currentHttpStringsHeadersSplitted[0], " ")[0]

				var headerStrings []string

				var msgPayload []string
				for _, headerString := range currentHttpStringsHeadersSplitted[1:] {
					if headerString != "" {
						headerStrings = append(headerStrings, headerString)
					}
				}

				headers := parseHeaders(headerStrings)

				if requestType == "POST" || strings.Contains(requestType, "HTTP") {
					// we will have content-length, because it's POST request or an HTTP response
					contentLength, _ := headers["Content-Length"]
					contentLengthVal, _ := strconv.Atoi(contentLength)
					fmt.Printf("Is contentLength longer than content?: %t %v %d", (len(strings.Split(msgString, "\r\n\r\n")[1]) < contentLengthVal), msgString, contentLengthVal)
					data := strings.Split(msgString, "\r\n\r\n")[1][:contentLengthVal]

					leftOver := strings.Split(msgString, data)[1]
					msgString = leftOver
					msgPayload = strings.Split(data, "\r\n")
				} else {
					msgString = ""
				}

				if strings.Contains(requestType, "HTTP") {
					// request is actually a response, do response
					doResponse(conn, currentHttpStringsHeadersSplitted, strings.Join(msgPayload, ""), headers, connType)
				} else {
					doRequest(conn, currentHttpStringsHeadersSplitted, strings.Join(msgPayload, ""), requestType, headers)
				}

				connectionType, ok := headers["Connection"]

				if ok {
					if strings.ToLower(connectionType) != "keep-alive" {
						// check remote addr
						fmt.Print(conn.RemoteAddr().String())

						mux.RLock()
						val, ok := existingClientToNodeMap[conn.RemoteAddr().String()]
						mux.RUnlock()

						if ok {
							// there is a node corresponding to the closed client connection, delete the connection
							_, doesNodeExist := existingNodesAddresses[val]
							mux.Lock()
							if doesNodeExist {
								delete(existingNodesAddresses, val)
							}

							delete(existingClientToNodeMap, conn.RemoteAddr().String())
							delete(existingNodeToClientMap, val)
							delete(existingClientsAddresses, conn.RemoteAddr().String()) // remove the client connection from map
							currentConnections--
							mux.Unlock()
							fmt.Printf("Deleted client obj %s", conn.RemoteAddr().String())
						}
						return // any non keep-alive connection type results in the connection being closed
					}
					continue
				} else {
					// Default to no keep-alive
					mux.RLock()
					val, ok := existingClientToNodeMap[conn.RemoteAddr().String()]
					mux.RUnlock()

					if ok {
						// there is a node corresponding to the closed client connection, delete the connection
						_, doesNodeExist := existingNodesAddresses[val]
						mux.Lock()
						if doesNodeExist {
							delete(existingNodesAddresses, val)
						}

						delete(existingClientToNodeMap, conn.RemoteAddr().String())
						delete(existingNodeToClientMap, val)
						delete(existingClientsAddresses, conn.RemoteAddr().String()) // remove the client connection from map
						currentConnections--
						mux.Unlock()
						fmt.Printf("Deleted client obj %s", conn.RemoteAddr().String())
					}
					return
				}
			}
		}
	}
}

var existingNodesAddresses = map[string]*Node{}
var existingClientToNodeMap = map[string]string{} // client ip -> node ip if the client was a distributed node
var existingNodeToClientMap = map[string]string{} // node ip -> client ip if the client was a distributed node
var transactions = map[string]*Transaction{}      // transaction hash -> transaction data
var existingClientsAddresses = map[string]*Client{}
var serverAddres string
var serverAddresHash string
var maxConnections int
var currentConnections = 0
var nBeforeHash = 2

var triedNodes = map[string]bool{}

var mux sync.RWMutex

var initialNodesAdresses = []string{}

func main() {
	f, err := os.OpenFile("serverlog.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err != nil {
		log.Fatalf("Error opening log file!: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	// log.SetFlags(log.Ldate | log.Ltime)

	// Load initial nodes
	mainNodesJsonFile, err := os.Open("main_nodes.json")

	if err != nil {
		log.Fatalf("Error opening JSON config file with hardcoded nodes! %v", err)
	}

	defer mainNodesJsonFile.Close()

	mainNodesJsonFileByteValue, _ := ioutil.ReadAll(mainNodesJsonFile)

	var initialNodes InitialNodes

	json.Unmarshal(mainNodesJsonFileByteValue, &initialNodes)

	// Parse loaded initial nodes
	for _, neighbourNodeString := range initialNodes.Nodes {
		splitted := strings.Split(neighbourNodeString, ":")

		initialNodesAdresses = append(initialNodesAdresses, splitted[0]+":"+splitted[1])
	}

	argsWithoutProg := os.Args[1:]

	port := "8080"
	ip := "0.0.0.0"
	maxConnectionsArg := 2

	if len(argsWithoutProg) > 0 {
		port = argsWithoutProg[0]
	}

	if len(argsWithoutProg) > 1 {
		ip = argsWithoutProg[1]
	}

	if len(argsWithoutProg) > 2 {
		maxConnectionsArg, _ = strconv.Atoi(argsWithoutProg[2])
	}

	maxConnections = maxConnectionsArg

	server := NewServer(ip + ":" + port)

	log.Fatal(server.Start())
}
