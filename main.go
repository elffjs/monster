package main

import (
	"context"
	"log"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

var one = big.NewInt(1)
var confirmations = big.NewInt(5)

type blockClient struct {
	eth *ethclient.Client
}

func (c *blockClient) SubscribeConfirmedBlocks(ctx context.Context, next *big.Int, ch chan<- *big.Int) *blockSubscription {
	sub := &blockSubscription{make(chan struct{})}
	go subscribeConfirmedBlocks(ctx, c.eth, next, ch, sub.done)
	return sub
}

type blockSubscription struct {
	done chan struct{}
}

func (s *blockSubscription) Wait() {
	<-s.done
}

// Note that next is mutated, the caller shouldn't use it after passing it in.
func subscribeConfirmedBlocks(ctx context.Context, client *ethclient.Client, next *big.Int, out chan<- *big.Int, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			head, err := client.BlockByNumber(ctx, nil)
			if err != nil {
				log.Printf("Failed to get head block: %v", err)
				continue
			}

			confirmedHead := new(big.Int).Sub(head.Number(), confirmations)

			for ; next.Cmp(confirmedHead) <= 0; next.Add(next, one) {
				// Have to copy. Otherwise we might mutate the value before it is used.
				out <- new(big.Int).Set(next)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	eth, err := ethclient.Dial("https://polygon.llamarpc.com")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	blocks := make(chan *big.Int)

	client := blockClient{eth: eth}
	sub := client.SubscribeConfirmedBlocks(ctx, big.NewInt(38448266), blocks)

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt)

Loop:
	for {
		select {
		case bl := <-blocks:
			log.Printf("Block %d", bl)
		case sig := <-stop:
			log.Printf("Got signal %s, terminating.", sig)
			break Loop
		}
	}

	cancel()
	sub.Wait()
}
