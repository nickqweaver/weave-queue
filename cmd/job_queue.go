package main

import queue "github.com/nickqweaver/job-queue/internal"

func main() {
	producer := queue.Producer{}
	for id := 1; id <= 100; id++ {
		producer.Enqueue("test", id)
	}

	consumer := queue.Consumer{}
	consumer.Run("test")
}
