package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/alphadose/haxmap"
)

type data struct {
	id  int
	exp time.Time
}

func main() {
	c := haxmap.New[int, *data](256)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		t := time.NewTicker(time.Second * 2)
		defer t.Stop()
		var count int
		for {
			select {
			case <-t.C:
				count = 0
				c.ForEach(func(s int, b *data) bool {
					if time.Now().After(b.exp) {
						c.Del(s)
						count++
					}
					return true
				})
				fmt.Println("Del", count)
			case <-ctx.Done():
				return
			}
		}
	}()

	for i := 0; i < 20000; i++ {
		c.Set(i, &data{id: i, exp: time.Now().Add(time.Millisecond * time.Duration((1000 + rand.Intn(800))))})
		time.Sleep(time.Microsecond * time.Duration(rand.Intn(200)+10))
		if i%100 == 0 {
			fmt.Println(i)
		}
	}

	time.Sleep(time.Second * 3)
	fmt.Println("LEN", c.Len())
}
