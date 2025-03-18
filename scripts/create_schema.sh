#!/bin/bash

# Create Block schema
curl -X POST http://127.0.0.1:9181/api/v0/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "mutation { create_schema(type: Block, collection: \"Block\", unique: [[\"number\", \"hash\"]]) { hash: String number: Int time: Int parentHash: String difficulty: String gasUsed: Int gasLimit: Int nonce: String miner: String size: Float stateRootHash: String uncleHash: String transactionRootHash: String receiptRootHash: String extraData: String } }"
  }'

# Create Transaction schema
curl -X POST http://127.0.0.1:9181/api/v0/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "mutation { create_schema(type: Transaction, collection: \"Transaction\", unique: [[\"hash\"]]) { hash: String blockHash: String blockNumber: Int from: String to: String value: String gas: Int gasPrice: String input: String nonce: Int transactionIndex: Int status: Boolean } }"
  }'

# Create Log schema
curl -X POST http://127.0.0.1:9181/api/v0/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "mutation { create_schema(type: Log, collection: \"Log\", unique: [[\"transactionHash\", \"logIndex\"]]) { address: String topics: [String] data: String blockNumber: Int transactionHash: String transactionIndex: Int blockHash: String logIndex: Int removed: Boolean } }"
  }'

# Create Event schema
curl -X POST http://127.0.0.1:9181/api/v0/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "mutation { create_schema(type: Event, collection: \"Event\", unique: [[\"transactionHash\", \"logIndex\"]]) { address: String topics: [String] data: String blockNumber: Int transactionHash: String transactionIndex: Int blockHash: String logIndex: Int name: String args: String removed: Boolean } }"
  }'
