package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

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

// Return the hash + timestamp (as iso string) of blocks on disk
func getBlocksHashTimestampOnDisk() [][]string {
	ledgerFileData, err := os.ReadFile(serverAddressHash + ".txt")

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
	ledgerFileData, err := os.ReadFile(serverAddressHash + ".txt")

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

func getMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 1 {
		return transactions[0].Hash
	}

	currentTransactionsList := []Transaction{}
	transactionsLen := len(transactions)

	i := 0
	for i < transactionsLen {
		currentTransaction := transactions[i]
		if i+1 >= transactionsLen {
			value := currentTransaction.Hash + currentTransaction.Hash

			currentTransactionsList = append(currentTransactionsList, Transaction{Hash: sha256encode([]byte(value))})
			break
		}

		nextTransaction := transactions[i+1]
		value := currentTransaction.Hash + nextTransaction.Hash

		currentTransactionsList = append(currentTransactionsList, Transaction{Hash: sha256encode([]byte(value))})

		i += 2
	}

	return getMerkleRoot(currentTransactionsList)
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
		if doesBlockAlreadyExist {
			// already got block with given hash, do not add it
			messageBack := Message{Success: false, ErrMsg: "Already got a block with identical data, aborted", Msg: "", MsgType: "block"}

			messageBackJsonBytes, _ := json.Marshal(messageBack) // cannot fail under normal circumstances

			res := HTTPResponse{status: ERROR500, headers: map[string]string{"Content-Type": CONTENT_JSON}, ctxHeaders: headers, data: string(messageBackJsonBytes)}

			conn.Write(res.buildBytes())

			return
		}

		if len(block.Hash) == 0 {
			messageBack := Message{Success: false, ErrMsg: "Empty block hash abort", Msg: "", MsgType: "block"}

			res := HTTPResponse{}
			res.setCtxHeaders(headers)
			res.setData(messageBack)
			res.setStatus(ERROR500)
			res.setHeader("Content-Type", CONTENT_JSON)

			conn.Write(res.buildBytes())

			return
		}
	}

	// write block to file
	// f, err := os.WriteFile(block.Hash + "_" + strconv.FormatInt(block.Timestamp.Unix(), 10) + ".txt")
	// ^ old style file save (a bit too obscure)

	if !doesBlockAlreadyExist {
		f, err := os.OpenFile(serverAddressHash+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

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
					blockToSend, _ := json.Marshal(block)

					fmt.Println(existingClient.conn.LocalAddr(), existingClient.conn.RemoteAddr().String())

					req := HTTPRequest{requestType: REQ_POST, path: "/blockReceive", version: VERSION1_1, data: string(blockToSend)}
					req.setHeader("X-Own-IP", serverAddress)

					// existingClient.doClientRequest("POST /blockReceive HTTP/1.1\r\nConnection:keep-alive\r\nX-Own-IP:" + serverAddress + "\r\n\r\n" + string(blockToSend))
					existingClient.conn.Write(req.buildBytes())
				}
			}
		}
		mux.Unlock()
	}

	if !isNode {
		messageBack := Message{Success: true, ErrMsg: "", Msg: "Made block and send it out", MsgType: "block"}

		res := HTTPResponse{}
		res.setData(messageBack)
		res.setStatus(OK200)
		res.setHeader("Content-Type", CONTENT_JSON)
		res.setCtxHeaders(headers)

		conn.Write(res.buildBytes())

		return
	}
}
