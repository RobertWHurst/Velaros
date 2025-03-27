package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/RobertWHurst/velaros"
)

func main() {
	router := velaros.NewRouter()

	router.Bind("/time/start", func(ctx *velaros.Context) {
		fmt.Println("Starting time")

		ctx.SetOnSocket("timeState", &TimeState{
			sendTime: true,
		})

		for {
			timeState := ctx.GetFromSocket("timeState").(*TimeState)
			if !timeState.ShouldSendTime() {
				break
			}

			fmt.Println("Sending time")
			err := ctx.Send(map[string]any{
				"time": time.Now().Unix(),
			})
			if err != nil {
				fmt.Println("Error sending time:", err)
				break
			}
			time.Sleep(1 * time.Second)
		}
	})

	router.Bind("/time/stop", func(ctx *velaros.Context) {
		fmt.Println("Stopping time")
		timeState := ctx.GetFromSocket("timeState").(*TimeState)
		timeState.Stop()
	})

	http.Handle("/", router)
	fmt.Println("Starting server on port 8167")
	err := http.ListenAndServe(":8167", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

type TimeState struct {
	mx       sync.Mutex
	sendTime bool
}

func (ts *TimeState) ShouldSendTime() bool {
	ts.mx.Lock()
	defer ts.mx.Unlock()
	return ts.sendTime
}

func (ts *TimeState) Stop() {
	ts.mx.Lock()
	defer ts.mx.Unlock()
	ts.sendTime = false
}
