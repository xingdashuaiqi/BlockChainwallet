package block

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"blockchain/utils"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fatih/color"
)

// db文件名
const dbFile = "blockchain.db"

// bucket名称
const blocksBucket = "blocks"
const MINING_DIFFICULT = 3
const MINING_ACCOUNT_ADDRESS = "CHEWANXING BLOCKCHAIN"
const MINING_REWARD = 5000
const MINING_TIMER_SEC = 10
const (
	//以下参数可以添加到启动参数
	BLOCKCHAIN_PORT_RANGE_START      = 5000
	BLOCKCHAIN_PORT_RANGE_END        = 5003
	NEIGHBOR_IP_RANGE_START          = 0
	NEIGHBOR_IP_RANGE_END            = 0
	BLOCKCHIN_NEIGHBOR_SYNC_TIME_SEC = 20
)

type Block struct {
	nonce             int
	blockid           *big.Int
	hash              [32]byte
	previousHash      [32]byte
	timestamp         int64
	transactions      []*Transaction
	blockchainAddress string
	port              uint16
}

func (b *Block) PreviousHash() [32]byte {
	return b.previousHash
}

func (b *Block) Nonce() int {
	return b.nonce
}

func (b *Block) Transactions() []*Transaction {
	return b.transactions
}

func (b *Block) Print() {
	log.Printf("%-15v:%30d\n", "timestamp", b.timestamp)
	//fmt.Printf("timestamp       %d\n", b.timestamp)
	log.Printf("%-15v:%30d\n", "nonce", b.nonce)
	log.Printf("%-15v:%30x\n", "previous_hash", b.previousHash)
	//log.Printf("%-15v:%30s\n", "transactions", b.transactions)
	for _, t := range b.transactions {
		t.Print()
	}
}

type Blockchain struct {
	transactionPool   []*Transaction
	chain             []*Block
	hash              []byte
	blockchainAddress string
	port              uint16
	mux               sync.Mutex
	neighbors         []string
	muxNeighbors      sync.Mutex
	tip               []byte
	db                *bolt.DB
}

// 迭代器结构体
type BlockchainIterator struct {
	currentHash []byte   // 当前区块hash
	db          *bolt.DB // 已经打开的数据库
}

// 检查区块链
func (bc *Blockchain) Iterator() *BlockchainIterator {
	return &BlockchainIterator{
		currentHash: bc.tip,
		db:          bc.db,
	}

}

// 拿到当前块
var ErrBlockNotFound = errors.New("block not found")

func (iter *BlockchainIterator) Next() (*Block, bool) {
	var block *Block
	//var prehash []byte
	color.Red("==================next: ", iter.currentHash)
	err := iter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockData := b.Get(iter.currentHash)
		dataString1 := fmt.Sprintf("%v", iter.currentHash)
		fmt.Println("currentHash:", dataString1)
		block = DeserializeBlock(blockData)

		return nil
	})

	if err != nil {
		log.Fatal("next error")
	}
	//copy(prehash[:], block.previousHash[:])
	iter.currentHash = block.previousHash[:]

	return block, !Equal(block.previousHash, [32]byte{})
}

func Equal(slice1, slice2 [32]byte) bool {
	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}
	return true
}

// 序列化区块
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	marshalJSON, _ := b.MarshalJSON()
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(marshalJSON)
	if err != nil {
		color.Red("序列化区块错误")
		log.Fatal(err)
	}
	color.Blue("序列化区块成功")
	return result.Bytes()
}

// 反序列化
func DeserializeBlock(d []byte) *Block {
	var block Block
	var result []byte
	// 编码器
	decoder := gob.NewDecoder(bytes.NewReader(d))
	// 编码
	err := decoder.Decode(&result)
	if err != nil {
		color.Red("解析区块错误")
		log.Fatal(err)

	}
	err = block.UnmarshalJSON(result)
	if err != nil {
		color.Red("解析JSON错误")
		log.Fatal(err)
	}
	return &block
}
func NewBlockchain(blockchainAddress string, port uint16) *Blockchain {
	var tip []byte
	bc := new(Blockchain)
	bc.blockchainAddress = blockchainAddress
	bc.port = port
	db, _ := bolt.Open(dbFile, 0600, nil)
	db.Update(func(tx *bolt.Tx) error {
		//获取bucket
		buck := tx.Bucket([]byte(blocksBucket))
		//如果没有链则创建创世块
		if buck == nil {
			color.Green(strings.Repeat("==", 5) + "创建创世块" + strings.Repeat("==", 5))
			genesis := NewGenesisBlock(0, blockchainAddress, port)
			//编码
			block_data := genesis.Serialize()
			bucket, _ := tx.CreateBucket([]byte(blocksBucket))
			bucket.Put(genesis.hash[:], block_data)
			bucket.Put([]byte("last"), genesis.hash[:])
			tip = genesis.hash[:]
		} else {
			tip = buck.Get([]byte("last"))

		}

		return nil
	})
	bc.db = db
	bc.tip = tip
	return bc
}

// 创世块
func NewGenesisBlock(nonce int, blockchainAddress string, port uint16) *Block {
	b := new(Block)
	b.timestamp = time.Now().UnixNano()
	b.nonce = nonce
	b.previousHash = [32]byte{}
	b.hash = b.Hash()
	b.transactions = []*Transaction{}
	b.blockid = big.NewInt(0)
	b.blockchainAddress = blockchainAddress
	b.port = port
	return b
}
func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}

func (bc *Blockchain) Run() {
	color.Red("==================run: ", bc.tip)
	bc.StartSyncNeighbors()
	bc.ResolveConflicts()
	bc.StartMining()
}

func (bc *Blockchain) SetNeighbors() {
	bc.neighbors = utils.FindNeighbors(
		utils.GetHost(), bc.port,
		NEIGHBOR_IP_RANGE_START, NEIGHBOR_IP_RANGE_END,
		BLOCKCHAIN_PORT_RANGE_START, BLOCKCHAIN_PORT_RANGE_END)

	color.Blue("邻居节点：%v", bc.neighbors)
}

func (bc *Blockchain) SyncNeighbors() {
	bc.muxNeighbors.Lock()
	defer bc.muxNeighbors.Unlock()
	bc.SetNeighbors()
}

func (bc *Blockchain) StartSyncNeighbors() {
	bc.SyncNeighbors()
	_ = time.AfterFunc(time.Second*BLOCKCHIN_NEIGHBOR_SYNC_TIME_SEC, bc.StartSyncNeighbors)
}

func (bc *Blockchain) TransactionPool() []*Transaction {
	return bc.transactionPool
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = bc.transactionPool[:0]
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*Block `json:"chain"`
	}{
		Blocks: bc.chain,
	})
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*Block `json:"chain"`
	}{
		Blocks: &bc.chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}
// 添加区块
func (bc *Blockchain) AddBlock(nonce int) *Block {
	var lastHash []byte
	//值类型转换
	var lastHash32 [32]byte
	var nowblock []byte
	_ = bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("last"))
		nowblock = b.Get(lastHash)
		return nil
	})
	// 将 []byte 直接转换为 [32]byte
	copy(lastHash32[:], lastHash)
	now := DeserializeBlock(nowblock)
	color.Red("noce: " + strconv.Itoa(now.nonce))
	color.Red("blockid: " + now.blockid.String())
	newBlock := NewBlock(nonce, now.hash, bc.transactionPool, now.blockid)
	_ = bc.db.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket([]byte(blocksBucket))
		_ = buck.Put(newBlock.hash[:], newBlock.Serialize())
		_ = buck.Put([]byte("last"), newBlock.hash[:])
		bc.tip = newBlock.hash[:]
		return nil
	})
	return newBlock
}
func NewBlock(nonce int, previousHash [32]byte, txs []*Transaction, blockid *big.Int) *Block {
	b := new(Block)
	b.timestamp = time.Now().UnixNano()
	b.nonce = nonce
	b.previousHash = previousHash
	b.transactions = txs
	b.hash = b.Hash()
	blockid.Add(blockid, big.NewInt(1))
	b.blockid = blockid
	return b
}
func (bc *Blockchain) Print() {

	for i, block := range bc.chain {
		color.Green("%s BLOCK %d %s\n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		block.Print()
	}
	color.Yellow("%s\n\n\n", strings.Repeat("*", 50))
}

func (b *Block) Hash() [32]byte {
	m, _ := json.Marshal(b)
	return sha256.Sum256([]byte(m))
}

func (b *Block) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		Timestamp         int64          `json:"timestamp"`
		Blockid           *big.Int       `json:"blockid"`
		Nonce             int            `json:"nonce"`
		PreviousHash      string         `json:"previous_hash"`
		Hash              string         `json:"hash"`
		Transactions      []*Transaction `json:"transactions"`
		BlockchainAddress string         `json:"blockchainAddress"`
		Port              uint16         `json:"port"`
	}{
		Timestamp:         b.timestamp,
		Blockid:           b.blockid,
		Nonce:             b.nonce,
		PreviousHash:      fmt.Sprintf("%x", b.previousHash),
		Hash:              fmt.Sprintf("%x", b.hash),
		Transactions:      b.transactions,
		BlockchainAddress: b.blockchainAddress,
		Port:              b.port,
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var previousHash string
	var hash string
	var blockid uint64
	v := &struct {
		Timestamp         *int64          `json:"timestamp"`
		Nonce             *int            `json:"nonce"`
		Blockid           *uint64         `json:"blockid"`
		PreviousHash      *string         `json:"previous_hash"`
		Hash              *string         `json:"hash"`
		Transactions      *[]*Transaction `json:"transactions"`
		BlockchainAddress *string         `json:"blockchainAddress"`
		Port              *uint16         `json:"port"`
	}{
		Timestamp:         &b.timestamp,
		Blockid:           &blockid,
		Nonce:             &b.nonce,
		PreviousHash:      &previousHash,
		Hash:              &hash,
		Transactions:      &b.transactions,
		BlockchainAddress: &b.blockchainAddress,
		Port:              &b.port,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	b.blockid = big.NewInt(int64(blockid))
	ph, _ := hex.DecodeString(*v.PreviousHash)
	copy(b.previousHash[:], ph)
	ph1, _ := hex.DecodeString(*v.Hash)
	copy(b.hash[:], ph1)

	return nil
}
func (bc *Blockchain) CreateTransaction(sender string, recipient string, value uint64,
	senderPublicKey *ecdsa.PublicKey, hash string, s *utils.Signature) bool {
	isTransacted := bc.AddTransaction(sender, recipient, uint64(value), senderPublicKey, hash, s)

	if isTransacted {
		for _, n := range bc.neighbors {
			publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(),
				senderPublicKey.Y.Bytes())
			signatureStr := s.String()
			bt := &TransactionRequest{
				&sender, &recipient, &publicKeyStr, &value, &hash, &signatureStr}
			m, _ := json.Marshal(bt)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("http://%s/transactions", n)
			client := &http.Client{}
			req, _ := http.NewRequest("PUT", endpoint, buf)
			resp, _ := client.Do(req)
			log.Printf("%v", resp)
		}
	}

	return isTransacted
}

func (bc *Blockchain) CopyTransactionPool() []*Transaction {
	transactions := make([]*Transaction, 0)
	for _, t := range bc.transactionPool {
		transactions = append(transactions,
			NewTransaction(t.senderAddress,
				t.receiveAddress,
				t.hash,
				t.value))
	}
	return transactions
}

func (bc *Blockchain) ValidProof(nonce int,
	previousHash [32]byte,
	transactions []*Transaction,
	difficulty int,
) bool {
	zeros := strings.Repeat("0", difficulty)
	tmpBlock := Block{nonce: nonce, previousHash: previousHash, transactions: transactions}
	tmpHashStr := fmt.Sprintf("%x", tmpBlock.Hash())
	return tmpHashStr[:difficulty] == zeros
}

func (bc *Blockchain) ProofOfWork() int {
	//获取当前块
	iter := bc.Iterator()
	block, _ := iter.Next()
	transactions := bc.CopyTransactionPool() //选择交易？控制交易数量？
	previousHash := block.hash
	nonce := 0
	begin := time.Now()
	for !bc.ValidProof(nonce, previousHash, transactions, MINING_DIFFICULT) {
		nonce += 1
	}
	end := time.Now()
	log.Printf("POW spend Time:%f Second", end.Sub(begin).Seconds())
	log.Printf("POW spend Time:%s Diff:%v", end.Sub(begin), MINING_DIFFICULT)
	log.Printf("POW spend Time:%s", end.Sub(begin))

	return nonce
}

// 将交易池的交易打包
func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	color.Red("==========Mining: ", bc.tip)
	nonce := bc.ProofOfWork()
	log.Println("nonce成功")
	block := bc.AddBlock(nonce)
	log.Println("创建block成功")
	log.Println("action=mining, status=success")
	flag := bc.AddTransaction(MINING_ACCOUNT_ADDRESS, bc.blockchainAddress, MINING_REWARD, nil, fmt.Sprintf("%x", block.Hash()), nil)
	log.Printf("action=add_transaction, status=%v\n", flag)

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		client := &http.Client{}
		req, _ := http.NewRequest("PUT", endpoint, nil)
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}

	return true
}

func (bc *Blockchain) CalculateTotalAmount(accountAddress string) uint64 {
	var totalAmount uint64 = 0
	bci := bc.Iterator()
	for {
		block, next := bci.Next()
		for _, transaction := range block.transactions {
			if accountAddress == transaction.receiveAddress {
				totalAmount = totalAmount + uint64(transaction.value)
			}
			if accountAddress == transaction.senderAddress {
				totalAmount = totalAmount - uint64(transaction.value)
			}
		}
		if !next {
			return totalAmount
		}
	}
}

func (bc *Blockchain) StartMining() {
	bc.Mining()
	_ = time.AfterFunc(time.Second*MINING_TIMER_SEC, bc.StartMining)
	color.Yellow("minetime: %v\n", time.Now())
}

type AmountResponse struct {
	Amount uint64 `json:"amount"`
}

func (ar *AmountResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount uint64 `json:"amount"`
	}{
		Amount: ar.Amount,
	})
}

type Transaction struct {
	senderAddress  string
	receiveAddress string
	hash           string
	value          uint64
}

func NewTransaction(sender string, receive string, hash string, value uint64) *Transaction {
	t := Transaction{sender, receive, hash, value}
	return &t
}
func (bc *Blockchain) AddTransaction(
	sender string,
	recipient string,
	value uint64,
	senderPublicKey *ecdsa.PublicKey,
	hash string,
	s *utils.Signature) bool {
	t := NewTransaction(sender, recipient, hash, value)

	//如果是挖矿得到的奖励交易，不验证
	if sender == MINING_ACCOUNT_ADDRESS {
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}
	if bc.VerifyTransactionSignature(senderPublicKey, s, t) {

		bc.transactionPool = append(bc.transactionPool, t)
		return true
	} else {
		log.Println("ERROR: 验证交易")
	}
	return false

}

// 通过区块号查区块
func (bc *Blockchain) GetBlockByNumber(id int) (*Block, error) {
	iter := bc.Iterator()
	for {
		block, next := iter.Next()
		if block.blockid.Cmp(big.NewInt(int64(id))) == 0 {
			return block, nil
		}
		if !next {
			return nil, errors.New("找不到区块")
		}
	}
}

// 通过hash查交易
func (bc *Blockchain) GetTransactionByHash(hash string) *Transaction {
	if len(hash) == 0 {
		return nil
	}
	iter := bc.Iterator()
	for {
		block, next := iter.Next()
		for _, transaction := range block.transactions {
			if transaction.hash == hash {
				return transaction
			}
		}
		if !next {
			return nil
		}
	}
}

// 通过hash查区块
func (bc *Blockchain) GetBlockByHash(hash string) (*Block, error) {
	if len(hash) == 0 {
		return nil, nil
	}
	iter := bc.Iterator()
	for {
		block, next := iter.Next()
		if fmt.Sprintf("%x", block.hash) == hash {
			return block, nil
		}
		if !next {
			return nil, errors.New("找不到区块")
		}
	}
}
func (bc *Blockchain) GetTransactionByUserHash(hash []byte) []*Transaction {
	if len(hash) == 0 {
		return nil
	}
	iter := bc.Iterator()
	var userTransaction []*Transaction
	for {
		block, next := iter.Next()
		for _, transaction := range block.transactions {
			if transaction.receiveAddress == string(hash) || transaction.senderAddress == string(hash) {
				userTransaction = append(userTransaction, transaction)
			}
		}
		if !next {
			return userTransaction
		}
	}

}
func (bc *Blockchain) VerifyTransactionSignature(
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256([]byte(m))
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
}

func (t *Transaction) Print() {
	color.Red("%s\n", strings.Repeat("~", 30))
	color.Cyan("发送地址             %s\n", t.senderAddress)
	color.Cyan("接受地址             %s\n", t.receiveAddress)
	color.Cyan("金额                 %d\n", t.value)

}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sender    string `json:"sender_blockchain_address"`
		Recipient string `json:"recipient_blockchain_address"`
		Value     uint64 `json:"value"`
		Hash      string `json:"hash"`
	}{
		Sender:    t.senderAddress,
		Recipient: t.receiveAddress,
		Value:     t.value,
		Hash:      t.hash,
	})
}

func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		Sender    *string `json:"sender_blockchain_address"`
		Recipient *string `json:"recipient_blockchain_address"`
		Value     *uint64 `json:"value"`
		Hash      *string `json:"hash"`
	}{
		Sender:    &t.senderAddress,
		Recipient: &t.receiveAddress,
		Value:     &t.value,
		Hash:      &t.hash,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) ValidChain(chain []*Block) bool {
	preBlock := chain[0]
	currentIndex := 1
	for currentIndex < len(chain) {
		b := chain[currentIndex]
		if b.previousHash != preBlock.Hash() {
			return false
		}

		if !bc.ValidProof(b.Nonce(), b.PreviousHash(), b.Transactions(), MINING_DIFFICULT) {
			return false
		}

		preBlock = b
		currentIndex += 1
	}
	return true
}

func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*Block = nil
	maxLength := len(bc.chain)

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/chain", n)
		resp, err := http.Get(endpoint)
		if err != nil {
			color.Red("                 错误 ：ResolveConflicts GET请求")
			return false
		} else {
			color.Green("                正确 ：ResolveConflicts  GET请求")
		}
		if resp.StatusCode == 200 {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			err1 := decoder.Decode(&bcResp)

			if err1 != nil {
				color.Red("                 错误 ：ResolveConflicts Decode")
				return false
			} else {
				color.Green("                正确 ：ResolveConflicts  Decode")
			}

			chain := bcResp.Chain()
			color.Cyan("   ResolveConflicts   chain len:%d ", len(chain))
			if len(chain) > maxLength && bc.ValidChain(chain) {
				maxLength = len(chain)
				longestChain = chain
			}
		}
	}

	color.Cyan("   ResolveConflicts   longestChain len:%d ", len(longestChain))

	if longestChain != nil {
		bc.chain = longestChain
		log.Printf("Resovle confilicts replaced")
		return true
	}
	log.Printf("Resovle conflicts not replaced")
	return false
}

type TransactionRequest struct {
	SenderBlockchainAddress    *string `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string `json:"recipient_blockchain_address"`
	SenderPublicKey            *string `json:"sender_public_key"`
	Value                      *uint64 `json:"value"`
	Hash                       *string `json:"hash"`
	Signature                  *string `json:"signature"`
}

func (tr *TransactionRequest) Validate() bool {
	if tr.SenderBlockchainAddress == nil ||
		tr.RecipientBlockchainAddress == nil ||
		tr.SenderPublicKey == nil ||
		tr.Value == nil ||
		tr.Signature == nil {
		return false
	}
	return true
}
