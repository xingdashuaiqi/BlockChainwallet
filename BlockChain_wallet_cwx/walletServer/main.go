package main

import (
	"flag"
	"fmt"
	"log"
)

func init() {
	log.SetPrefix("Wallet Server: ")
}

func main() {
	port := flag.Uint("port", 8080, "TCP Port Number for Wallet Server")
	gateway := flag.String("gateway", "http://127.0.0.1:5000", "Blockchain Gateway")
	flag.Parse()
	fmt.Printf("port::%v gateway:%v\n", *port, *gateway)
	app := NewWalletServer(uint16(*port), *gateway)
	app.Run()
}

// ===矿工帐号信息====
// 矿工private_key
// 18e3d4fe357c5d9bb8e9e8cbadc456f5ed8810550ce9f0061e8c8db911b651f1
// 矿工publick_key
// 916641e9ef2e8130466318c9fc5ae26af61d9029cb47365c6de5a615e81283224f76cf26d7bdfbf2be0d4f0e158a2dc1947e18ea9533f1d1edd2ad55a96bee9a
// 矿工blockchain_address
// DxS8s2sDGnDYw6A7spN4gdYmQKiNs9pG59BhX5kj4Hpy
// ===============