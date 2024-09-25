package main

import (
	"encoding/json"
	"fmt"
	"log"
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
