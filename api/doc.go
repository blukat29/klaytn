// Copyright 2018 The klaytn Authors
// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from internal/ethapi/api.go (2018/06/04).
// Modified and improved for the klaytn development.

/*
Package api implements the general Klaytn API functions.

# Overview of api package

This package provides various APIs to access the data of the Klaytn node.
Remote users can interact with Klyatn by calling these APIs instead of IPC.
APIs are grouped by access modifiers (public or private) and namespaces (eth, klay, txpool, debug, personal).

Public eth_ APIs
  - api_ethereum.go: Ethereum compatible APIs (e.g. eth_getBlockByNumber)

Private personal_ APIs
  - api_private_account.go: Accounts managed by the node (e.g. personal_unlockAccount)

Public klay_ APIs
  - api_public_account.go: Accounts managed by the node (e.g. klay_accounts)
  - api_public_blockchain.go: Block data (e.g. klay_getBlockByNumber)
  - api_public_cypress.go: Cypress credit contract (e.g. klay_getCypressCredit)
  - api_public_klay.go: Klaytn specific data (e.g. klay_gasPrice)
  - api_public_transaction_pool.go: Transaction data (e.g. klay_getTransactionByHash)

Private debug_ APIs
  - api_private_debug.go: Debug data (e.g. debug_setHead, debug_chaindbProperty)

Public debug_ APIs
  - api_public_debug.go: Debug data (e.g. debug_getBlockRlp)
  - Note that debug_ APIs are also defined in other packages including api/debug, node/cn, and node/cn/tracers

Public net_ APIs
  - api_public_net.go: P2P network related data (e.g. net_peerCount)

Public txpool_ APIs
  - api_public_tx_pool.go: Transaction pool related data (e.g. txpool_status)

Helper functions
  - addrlock.go: Addrlocker which prevents another tx getting the same nonce through API
  - backend.go: common API services
  - tx_args.go: common API argument types
*/
package api
