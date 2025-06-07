package main

import (
	"fmt"
	"net/http"
	_ "net/http"
	"sync"
	_ "sync"
	"time"
	_ "time"
)

// ScrapeResult holds all the information from scraping a single URL
// Using a struct allow us to pass around all related data as one unit
// This is especially important for concurrent programming where we need to
// send complete results through channels between goroutines
type ScrapeResult struct {
	URL           string        // the url that was scraped
	StatusCode    int           // HTTP status code (200, 404, 500, etc)
	ContentLength int64         // size of response body in bytes (-1 if unknown)
	Duration      time.Duration // How long the request took
	Error         error         // Any error that occurred (nil if successful)
}

// scrapeURL takes a URL a returns basic information about the response
// this demonstrates basic HTTP client usage and error handling patters in go
// This will return a ScrapedResult struct, this makes the function more flexible - called can decide what to do with results
// Return structured data is essential for concurrent processing
func scrapeURL(url string) ScrapeResult {
	// time.Now() captures the current timestamp - we'll use this to measure
	// how long the HTTP request takes (useful for performance analysis)
	start := time.Now()

	// http.Get() makes an HTTP Get request and returns two values:
	// 1. *http.Response (the response object)
	// 2. error (nil if successful, error object if something went wrong)
	// this is Go's standard error handling patter - always check the error!
	resp, err := http.Get(url)
	if err != nil {
		// If there's an error, return a result struct with the error
		// Notice we calculate duration even for failed requests
		return ScrapeResult{
			URL:      url,
			Duration: time.Since(start),
			Error:    err,
			// StatusCode and ContentLength will be zero values (0, 0)
		}
	}

	// defer means "execute this when the function exits, no matter how it exits"
	// res.Body is an io.ReadCloser - it holds the connection open until closed
	// Forgetting to close response bodies causes memory leaks and connection exhaustion
	// defer is perfect here because it guarantees cleanup even if we return early
	defer resp.Body.Close()

	// Print comprehensive information about the http response:
	// - StatusCode: HTTP status (200=ok, 404=not-found, 500=Server Error, etc.)
	// - ContentLength: size of the response body in bytes (-1 if unknown)
	// - duration: how long the request took (useful for performance comparison)
	// fmt.Printf("URL: %s | status: %d | Length: %d bytes | Time: %v\n",
	// 	url, resp.StatusCode, resp.ContentLength, duration)

	return ScrapeResult{
		URL:           url,
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		// time.Since() calculates the elapsed time from start until now
		// this measures the total time for the HTTP request (network + server processing)
		Duration: time.Since(start),
		Error:    nil, // Explicitly set to nil to show success
	}

}

// printResult displays a ScrapeResult in a readable format
// Separating display logic from scraping Logic follows good design principles
func printResult(result ScrapeResult) {
	if result.Error != nil {
		// Handle error case - show URL and error message
		fmt.Printf("ERROR - URL: %s | Error: %v | Time: %v\n",
			result.URL, result.Error, result.Duration)
	} else {
		// Handle success case - show all details
		fmt.Printf("SUCCESS - URL:%s | Status: %d | Length: %d bytes | Time: %v\n",
			result.URL, result.StatusCode, result.ContentLength, result.Duration)
	}
}

// worker is a goroutine that processes URLs from urlChannel and sends results to resultChannel
// This is the core of the worker pool pattern - multiple workers can run this function concurrently
// Each worker operates independently but shares the same input and output channels
func worker(id int, urlChannel <-chan string, resultChannel chan<- ScrapeResult, wg *sync.WaitGroup) {
	// sync.WaitGroup is used to wait for a collection of goroutines to finish
	// defer wg.Done() ensures we signal completion even if the function exits early
	// This is crucial for preventing deadlocks in the shutdown sequence

	defer wg.Done()

	// Channel direction annotations:
	// <-chan string means "receive-only channel" - this worker can only read URLS
	// chan<- ScrapeResult means "Send only channel" - this worker can only send results
	// THis prevents accidentally channels in the wrong direction
	fmt.Printf("Worker %d started\n", id)

	// Keep reading URLs from the channel until it's closed
	// This loop will process URLs as they become available
	// Multiple workers can compete for URLs from the same channel (fan-out pattern)
	for url := range urlChannel {
		fmt.Printf("Worker %d processing: %s\n", id, url)

		// Scrape the URL (this is the time-consuming operation)
		result := scrapeURL(url)

		// Send the result to the result channel
		// Other goroutines can receive these results (fan-in pattern)
		resultChannel <- result

		fmt.Printf("Worker %d finished: %s\n", id, url)
	}

	fmt.Printf("Worker %d finished - no more URLs\n", id)
}

// resultCollector is a dedicated goroutine for collecting and processing results
// this separates the concern of result collection from the main application logic
// It also allows for more sophisticated result processing (sorting, filtering, etc)
func resultCollector(resultChannel <-chan ScrapeResult, wg *sync.WaitGroup, done chan<- []ScrapeResult) {
	fmt.Println("Result collector started")

	var results []ScrapeResult
	resultCount := 0

	// Collect results until the channel is closed
	// Using range automatically handles channel closing - loop exists when channel closes
	// This is much cleaner that counting expected results manually
	for result := range resultChannel {
		resultCount++
		results = append(results, result)

		// Display result immediately as it arrives
		fmt.Printf("[%d] ", resultCount)
		printResult(result)
	}

	fmt.Printf("Result collector finished - collected %d results\n", len(results))

	// send the complete results back to the main goroutine
	// this allows main to continue summary processing
	done <- results
}

func main() {
	fmt.Println("Web scraper starting...")

	// Create a slice (dynamic array) of URLs to scrape
	// Using httpbin.org endpoints with different delays to simulate real-world variety
	// httpbin.org/delay/N waits N seconds before responding - perfect for testing
	urls := []string{
		"https://httpbin.org/delay/1",    // 1 second delay
		"https://httpbin.org/delay/2",    // 2 second delay
		"https://httpbin.org/delay/1",    // 1 second delay
		"https://httpbin.org/get",        // immediate response
		"https://httpbin.org/delay/3",    // 3 second delay
		"https://httpbin.org/status/200", // immediate response with 200 status
		"https://httpbin.org/delay/1",    // 1 second delay
		"https://httpbin.org/json",       // immediate JSON response
	}

	// Configure the worker pool
	// numWorkers determines how many goroutines will process URLs concurrently
	// More workers = more concurrency, but also more resource usage
	// Sweet spot is usually around number of CPU cores, but for I/O bound work (like HTTP requests)
	// we can use more workers since they spend most time waiting for network responses
	numWorkers := 4
	fmt.Printf("Starting worker pool with %d workers\n", numWorkers)

	// Create a channel to pass URLs between goroutines
	// make(chan string) creates a channel that can send/receive strings
	// Channels are Go's way of communicating between goroutines - "Don't communicate by sharing memory; share memory by communicating"
	// THis is an unbuffered channel - sends block until someone receives
	urlChannel := make(chan string)

	// Create a channel to collect results
	// This channel will carry ScrapeResult structs from workers back to main
	resultChannel := make(chan ScrapeResult)

	// Final results: collector -> main
	done := make(chan []ScrapeResult)

	// Create WaitGroup to track worker completion
	// WaitGroup is like a counter: Add() increments, Done() decrements, Wait() blocks until zero
	// This ensures we don't exit before all workers have finished their cleanup
	var wg sync.WaitGroup

	// Record start time for the entire scarping operation
	// This will help us measure total time for sequential processing
	totalStart := time.Now()

	go func() {
		// THis is an anonymous function (lambda) that runs in its own goroutine
		fmt.Println("URL feeder started...")

		// Send each URL to the channel
		for i, url := range urls {
			fmt.Printf("Feeding URL: %d/%d: %s\n", i+1, len(urls), url)

			// urlChannel <- url sends the URL to the channel
			// This will block if no one is reading from the channel (with unbuffered channels)
			urlChannel <- url
		}

		close(urlChannel)
		fmt.Println("URL feeder finished - channel closed")
	}()

	// Start the result collector goroutine
	// THis will collect results from workers and send them back when complete
	go resultCollector(resultChannel, &wg, done)

	// // Start a single worker goroutine
	// // The worker will process URLs from urlChannel and send results to resultChannel
	// go worker(1, urlChannel, resultChannel)

	// Start multiple worker goroutines
	// Each worker runs the same function but with a different ID for identification
	// All workers share the same urlChannel (input) and resultChannel (output)
	// This creates a worker pool where work is automatically distributed
	for i := 1; i <= numWorkers; i++ {
		// Add 1 to WaitGroup for each worker we're about to start
		// This tells the WaitGroup to expect one more goroutine to call Done()
		wg.Add(1)

		// Start the worker, passing the WaitGroup pointer
		// Each worker will call wg.Done() when it finishes (via defer)
		go worker(i, urlChannel, resultChannel, &wg)
	}

	// Add a goroutine to close the result channel when all workers finish
	// This goroutine coordinates the workers but doesn't send data itself
	go func() {
		// Wait for all workers to complete
		wg.Wait()

		// Now it's safe to close the result channel because all senders (workers) are done
		// This follows the "senders close channels" rule properly
		close(resultChannel)
		fmt.Println("All workers finished - result channel closed by coordinator")
	}()

	// // Collect results from the worker
	// fmt.Printf("\n=== Collecting Results from the Worker ===\n")
	// // Create a slice to store all results
	// // THis demonstrates collecting structured data instead of just printing
	// var results []ScrapeResult

	// for range urls {
	// 	// Receive a result from the worker
	// 	// This will block until a result is available
	// 	result := <-resultChannel

	// 	results = append(results, result)
	// 	printResult(result)
	// }

	// range over a channel receives values until the channel is closed
	// this is the idiomatic way to consume all values from a channel
	// the loop automatically ends when the channel is closed
	// for url := range urlChannel {
	// 	fmt.Printf("Received URL from channel: %s\n", url)

	// 	// Process the URL (same as before)
	// 	result := scrapeURL(url)
	// 	results = append(results, result)
	// 	printResult(result)

	// 	// Send result to result channel (we'll use this more in later steps)
	// 	// resultChannel <- result
	// }

	// Sequential processing: scrape each URL one after another
	// This is the "baseline" - each request waits for the previous one to complete
	// Notice how this wil take at least 1+2+1+0+3+0+1+0 = 8+ seconds total
	// fmt.Println("\n=== Sequential Scraping ===")
	// for i, url := range urls {
	// 	// The range keyword give us index (i) and value (url) for each element
	// 	// We'll use the index to show the progress through our list
	// 	fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(urls), url)

	// 	// call scrapeURL and collect the result
	// 	result := scrapeURL(url)

	// 	// Add the result to our collection
	// 	results = append(results, result)

	// 	// Display the result immediately
	// 	printResult(result)
	// }

	fmt.Println("\n=== Waiting for Results from Worker Pool ===")

	// Block until the result collector send us the complete results
	// This is a clean way to wai for all concurrent work to complete
	results := <-done

	// Calculate and display total time for sequential approach
	// time.since() give us the elapsed time from totalStart until now
	totalDuration := time.Since(totalStart)

	// Display summary statistics
	fmt.Printf("\n=== Worker Pool Summary ===\n")
	fmt.Printf("Number of workers: %d\n", numWorkers)
	fmt.Printf("Total URLs processed: %d\n", len(results))
	fmt.Printf("Total time: %v\n", totalDuration)

	// Calculate expected sequential time for comparison
	// This helps us understand the speedup gained from concurrency
	totalDelayTime := 1 + 2 + 1 + 0 + 3 + 0 + 1 + 0 // sum of httpbin delays
	fmt.Printf("Expected sequential time: ~%ds (sum of delays)\n", totalDelayTime)

	// Count successful vs failed requests
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}
	fmt.Printf("Successful requests: %d\n", successCount)
	fmt.Printf("Failed requests: %d\n", len(results)-successCount)

	// Calculate approximate speedup
	// Real speedup will be less than perfect due to network overhead and coordination costs
	if totalDuration.Seconds() > 0 {
		speedup := float64(totalDelayTime) / totalDuration.Seconds()
		fmt.Printf("Approximate speedup: %.2fx\n", speedup)
	}

	// Additional analysis: show processing time distribution
	fmt.Printf("\n=== Processing Time Analysis ===\n")
	var totalProcessingTime time.Duration
	for _, result := range results {
		totalProcessingTime += result.Duration
	}
	fmt.Printf("Total processing time (sum of all requests): %v\n", totalProcessingTime)
	fmt.Printf("Wall clock time (concurrent execution): %v\n", totalDuration)
	parallelismEfficiency := float64(totalProcessingTime) / float64(totalDuration*time.Duration(numWorkers))
	fmt.Printf("Parallelism efficiency: %.2f%% (how well we utilized %d workers)\n",
		parallelismEfficiency*100, numWorkers)
}
