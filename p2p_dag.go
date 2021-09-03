// ToDo
// Convert to Graph
// 	Store data as what?
// 	Store the whole graph in mem
//	 	To have a proper DB Engine. Not as graph datastructure in memory
//
// Add consensus
// Add Wallet

package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// for converting the data into a graph
	"github.com/cheekybits/genny/generic"
	"github.com/davecgh/go-spew/spew"
	golog "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"

	// net "github.com/libp2p/go-libp2p-net"
	net "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// Graph starts here
// Item the type of the binary search tree
type Item generic.Type

// Node a single node that composes the tree
type Node struct {
	// value         Item
	MCIndex       int    `json:"MCIndex"`
	Timestamp     string `json:"Timestamp"`
	Data          string `json:"Data"`
	Hash          string `json:"Hash"`
	PrevLeftHash  string `json:"PrevLeftHash"`
	PrevRightHash string `json:"PrevRightHash"`
}

func (n Node) MarshalText() (text []byte, err error) {
	type x Node
	return json.Marshal(x(n))
}

func (n *Node) UnmarshalText(text []byte) error {
	type x Node
	return json.Unmarshal(text, (*x)(n))
}

func (n *Node) String() string {
	return fmt.Sprintf("%s", n.Data)
}

// ItemGraph the Items graph
type ItemGraph struct {
	Nodes []*Node          `json:"nodes"`
	Edges map[Node][]*Node `json:"edges"`
	Lock  sync.RWMutex     `json:"lock"`
}

var Graph = ItemGraph{}

// AddNode adds a node to the graph
func (g *ItemGraph) AddNode(n *Node) {
	g.Lock.Lock()
	g.Nodes = append(g.Nodes, n)
	g.Lock.Unlock()
}

// AddEdge adds an edge to the graph
func (g *ItemGraph) AddEdge(n1, n2 *Node) {
	g.Lock.Lock()
	if g.Edges == nil {
		g.Edges = make(map[Node][]*Node)
	}
	g.Edges[*n1] = append(g.Edges[*n1], n2)
	g.Edges[*n2] = append(g.Edges[*n2], n1)
	g.Lock.Unlock()
}

// String prints the Graph
func (g *ItemGraph) String() {
	g.Lock.RLock()
	s := ""
	for i := 0; i < len(g.Nodes); i++ {
		s += g.Nodes[i].String() + " -> "
		near := g.Edges[*g.Nodes[i]]
		for j := 0; j < len(near); j++ {
			s += near[j].String() + " "
		}
		s += "\n"
	}
	// fmt.Println(s)
	// log.Printf(s)
	g.Lock.RUnlock()
}

// GetLength adds a node to the graph
func (g *ItemGraph) GetLength() int {
	g.Lock.RLock()
	glength := len(g.Nodes)
	g.Lock.RUnlock()
	// log.Printf("Lenght: %i", glength)
	return glength
}

var mutex = &sync.Mutex{}

// Have a tip prediction algo to get the prev multiple tips
//
// Support multiple tips
// Stop it from getting killed. Check the input for CTRL C

// Where to store McIndex
// Ephemeral data similar to the state Trie in Eth
// 	What is it?
// 	How to maintain the whole dag state? To get balances, headers so that thin clients can send next node
// 	Has only finalised nodes
// Node can be dangling as well. What purpose do they serve? IoT can be

// How are transactions executed?

// 	How to implement logic for node finalisation?
// 		If mainchain nodes only execute all transactions after finalisation, won't it bottleneck
// 		Transmit the finalization data to all nodes
// 		The node which approved the trans should execute it
// 		Else the node above and so on till the main chain
// limit node traffic. WE already verify only a finite amount.
// But IoT may also verify only a few and they may have dangling nodes which might be deleted
// Delegated PoS / trust to rate limit

// Include smart contracts
// Store as an LPG
// 	Query for prev nodes based on index

// how to integrate with kube?
// Kube as a manager of multiple nodes
// replication to multiple nodes
// sharding
// job vs workload
// pubsub equivalent
// how to connect public with private? both ways? Is private - public the concept of oracle?

func isNodeValid(newNode, oldNode Node) bool {
	// if oldBlock.Index+1 != newBlock.Index {
	// 	return false
	// }

	// Do a BFS and get the nodes attached to the newNode
	if oldNode.Hash != newNode.PrevLeftHash {
		return false
	}
	// if PrevRightNode.Hash != newNode.PrevRightHash {
	// 	return false
	// }

	if calculateHash(newNode) != newNode.Hash {
		return false
	}

	return true
}

// SHA256 hashing
func calculateHash(node Node) string {
	record := strconv.Itoa(node.MCIndex) + node.Timestamp + node.Data + node.PrevLeftHash + node.PrevRightHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// func selectTips(g ItemGraph) (Node, Node) {

// 	return oldNode, oldNode
// }

// create a new block using previous block's hash
func generateNode(oldNode Node, Data string) Node {

	var newNode Node

	t := time.Now()

	newNode.MCIndex = oldNode.MCIndex + 1
	newNode.Timestamp = t.String()
	newNode.Data = Data
	newNode.PrevLeftHash = oldNode.Hash
	// newNode.PrevLeftHash = oldNode.Hash
	newNode.Hash = calculateHash(newNode)

	return newNode
}

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeBasicHost(listenPort int, secio bool, randseed int64) (host.Host, error) {

	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	// Generate a key pair for this host. We will use it
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
	}

	// if !secio {
	// 	opts = append(opts, libp2p.NoEncryption())
	// }

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	if secio {
		log.Printf("Now run \"go run main.go -l %d -d %s -secio\" on a different terminal\n", listenPort+1, fullAddr)
	} else {
		log.Printf("Now run \"go run main.go -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)
	}

	return basicHost, nil
}

func handleStream(s net.Stream) {

	log.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go readData(rw)
	go writeData(rw)

	// stream 's' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {

	for {
		// log.Printf("in the read data loop")
		str, err := rw.ReadString('\n')
		if err != nil {
			// This is entered if any of the nodes are killed
			// ToDo: Change this
			log.Printf("error in read string")
			log.Fatal(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {

			// chain := make([]Block, 0)
			graph := ItemGraph{}
			if err := json.Unmarshal([]byte(str), &graph); err != nil {
				// log.Printf("\n-----\nError in unmarshalling into a byte array\n")
				log.Fatal(err)
			}

			// log.Printf("\n-----\nrecieved dag")
			// log.Printf("String graph:\n %v \n", str)
			// spew.Dump(graph)
			// log.Printf("\n-----\n")
			// log.Printf("%d -- %d", graph.GetLength(), Graph.GetLength())

			mutex.Lock()
			if graph.GetLength() > Graph.GetLength() {
				log.Printf("input graph is greater")
				Graph = graph
				bytes, err := json.MarshalIndent(Graph, "", "  ")
				if err != nil {
					log.Printf("Error in marshalling new graph")
					log.Fatal(err)
				}
				// Green console color: 	\x1b[32m
				// Reset console color: 	\x1b[0m
				fmt.Printf("\x1b[32m%s\x1b[0m> ", string(bytes))
				spew.Dump(graph)
			}
			mutex.Unlock()
		}
	}
}

func writeData(rw *bufio.ReadWriter) {

	go func() {
		for {
			// log.Printf("in the go func loop")
			time.Sleep(5 * time.Second)
			mutex.Lock()
			bytes, err := json.Marshal(Graph)
			if err != nil {
				// log.Printf("Error in marshalling graph in go func")
				log.Println(err)
			}
			mutex.Unlock()

			// bytes2, err := json.MarshalIndent(Graph, "", "  ")
			// if err != nil {
			// log.Printf("Error in marshalling new graph")
			// log.Fatal(err)
			// }
			// log.Printf("\n-- marshalled graph---\n %v ", bytes)
			// log.Printf("\n-- unmarshalled graph---\n %v \n\n", bytes2)

			mutex.Lock()
			rw.WriteString(fmt.Sprintf("%s\n", string(bytes)))
			rw.Flush()
			mutex.Unlock()

		}
	}()

	stdReader := bufio.NewReader(os.Stdin)

	for {
		// log.Printf("in the write data loop")
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		sendData = strings.Replace(sendData, "\n", "", -1)
		// bpm, err := strconv.Atoi(sendData)
		// if err != nil {
		// 	log.Fatal(err)
		// }

		// Find some way to ge tthe last added node
		prevNode := Graph.Nodes[Graph.GetLength()-1]
		newNode := generateNode(*prevNode, sendData)

		if isNodeValid(newNode, *prevNode) {
			mutex.Lock()
			Graph.AddNode(&newNode)
			Graph.AddEdge(&newNode, prevNode)
			mutex.Unlock()
		}

		bytes, err := json.Marshal(Graph)
		if err != nil {
			// log.Printf("Error in marshalling graph in writedata")
			log.Println(err)
		}

		spew.Dump(Graph)

		mutex.Lock()
		rw.WriteString(fmt.Sprintf("%s\n", string(bytes)))
		rw.Flush()
		mutex.Unlock()
		// log.Printf("%s\n", string(bytes))
		// log.Printf("end of write data loop")
	}

}

func main() {
	t := time.Now()
	genesisNode := Node{}
	genesisNode = Node{0, t.String(), "Genesis", calculateHash(genesisNode), "", ""}

	Graph.AddNode(&genesisNode)
	Graph.String()

	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	lvl, err := golog.LevelFromString("error")
	if err != nil {
		panic(err)
	}
	golog.SetAllLoggers(lvl)
	// golog.SetAllLoggers(gologging.INFO) // Change to DEBUG for extra info

	// Parse options from the command line
	listenF := flag.Int("l", 0, "wait for incoming connections")
	target := flag.String("d", "", "target peer to dial")
	secio := flag.Bool("secio", false, "enable secio")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	flag.Parse()

	if *listenF == 0 {
		log.Fatal("Please provide a port to bind on with -l")
	}

	// Make a host that listens on the given multiaddress
	ha, err := makeBasicHost(*listenF, *secio, *seed)
	if err != nil {
		log.Printf("error in basic host")
		log.Fatal(err)
	}

	if *target == "" {
		log.Println("listening for connections")
		// Set a stream handler on host A. /p2p/1.0.0 is
		// a user-defined protocol name.
		ha.SetStreamHandler("/p2p/1.0.0", handleStream)

		select {} // hang forever
		/**** This is where the listener code ends ****/
	} else {
		ha.SetStreamHandler("/p2p/1.0.0", handleStream)

		// The following code extracts target's peer ID from the
		// given multiaddress
		ipfsaddr, err := ma.NewMultiaddr(*target)
		if err != nil {
			log.Printf("Multiadrr error")
			log.Fatalln(err)
		}

		pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
		if err != nil {
			log.Printf("ipfsaddr error")
			log.Fatalln(err)
		}

		peerid, err := peer.IDB58Decode(pid)
		if err != nil {
			log.Printf("peer IDB58")
			log.Fatalln(err)
		}

		// Decapsulate the /ipfs/<peerID> part from the target
		// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
		targetPeerAddr, _ := ma.NewMultiaddr(
			fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
		targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

		// We have a peer ID and a targetAddr so we add it to the peerstore
		// so LibP2P knows how to contact it
		ha.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)

		log.Println("opening stream")
		// make a new stream from host B to host A
		// it should be handled on host A by the handler we set above because
		// we use the same /p2p/1.0.0 protocol
		s, err := ha.NewStream(context.Background(), peerid, "/p2p/1.0.0")
		if err != nil {
			log.Printf("Newstream error")
			log.Fatalln(err)
		}
		// Create a buffered stream so that read and writes are non blocking.
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

		// Create a thread to read and write data.
		// log.Printf("before go r/w")
		go writeData(rw)
		go readData(rw)
		// log.Printf("after go r/w")

		select {} // hang forever

	}
}
