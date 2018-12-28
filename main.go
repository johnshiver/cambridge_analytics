package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

func createDemux(tweetChan chan *twitter.Tweet) *twitter.SwitchDemux {
	demux := twitter.NewSwitchDemux()
	demux.Tweet = func(tweet *twitter.Tweet) {
		tweetChan <- tweet
	}
	return &demux
}

func getLocations(twitterClient *twitter.Client) {
	locations, _, err := twitterClient.Trends.Available()
	if err != nil {
		log.Fatal(err)
	}
	if len(locations) > 0 {
		for _, location := range locations {
			spew.Dump(location)
		}
	}
}

func extractFromUserTweets(userTweets []string) ([]string, []string) {
	hashTags := []string{}
	mentions := []string{}
	for _, tweet := range userTweets {
		words := strings.Split(tweet, " ")
		for _, word := range words {
			if strings.HasPrefix(word, "#") {
				hashTags = append(hashTags, word)
			}
			if strings.HasPrefix(word, "@") {
				mentions = append(mentions, word)
			}
		}
	}
	return hashTags, mentions

}

func printSortedCount(itemCounts map[string]int, threshold int) {

	const MIN_COUNT = 2

	counts := []int{}
	countItems := make(map[int][]string)
	for item, count := range itemCounts {
		countItems[count] = append(countItems[count], item)
	}

	for count := range countItems {
		counts = append(counts, count)
	}

	sort.Ints(counts)
	for _, i := range counts {
		if i < MIN_COUNT {
			continue
		}
		fmt.Println(strings.Repeat("-", 45))
		fmt.Println(i)
		fmt.Println(strings.Repeat("-", 45))
		for _, item := range countItems[i] {
			fmt.Println(item)
		}
	}
}

func printUserCounts(counts []int, countUser map[int][]string) {
	fmt.Println(strings.Repeat("#", 90))
	fmt.Println("User's tweet counts")
	sort.Ints(counts)
	for _, i := range counts {
		fmt.Println(strings.Repeat("-", 45))
		fmt.Println(i)
		fmt.Println(strings.Repeat("-", 45))
		for _, u := range countUser[i] {
			fmt.Println(u)
		}
	}
}

func analyzeUserTweets(counts []int, countUser map[int][]string, globalUserTweets map[string][]string) (map[string]int, map[string]int) {
	hashTagCounts := make(map[string]int, 0)
	mentionCounts := make(map[string]int, 0)
	for _, i := range counts {
		for _, u := range countUser[i] {
			userTweets := globalUserTweets[u]
			hashTags, mentions := extractFromUserTweets(userTweets)
			for _, hashTag := range hashTags {
				hashTagCounts[hashTag]++
			}
			for _, mention := range mentions {
				mentionCounts[mention]++
			}
		}
	}
	return hashTagCounts, mentionCounts
}

// returns the count values that are worth looking at
func getUserCounts(countUser map[int][]string, minutes int) []int {

	counts := []int{}
	for count := range countUser {
		if count > minutes/5 && count > 1 {
			counts = append(counts, count)
		}
	}
	return counts
}

func initApp() *twitter.Client {

	token, config := GetTokens()
	authHTTPClient := config.Client(oauth1.NoContext, token)
	twitterClient := twitter.NewClient(authHTTPClient)
	return twitterClient

}

func handleTweets(wg *sync.WaitGroup, tweetChan chan *twitter.Tweet) {

	checkTweetCountFrequency := time.Second * 20
	checkUserFrequency := time.Second * 20
	userTicker := time.NewTicker(checkUserFrequency)
	checkTweetCount := time.NewTicker(checkTweetCountFrequency)

	defer wg.Done()

	// tweet tracking variables --------------------------------------------------
	// probably bad design, but these are datastructures
	// that are shared between cases.
	// TODO use sync.Map for these...mb can take advantage
	// of more goroutines
	globalUsers := make(map[string]int, 0)
	globalUserTweets := make(map[string][]string)
	timeElapsed := time.Second
	tweetCount := 0
	// ------------------------------------------------------------------------------

	keepGoing := true
	for keepGoing {
		select {
		case newTweet, ok := <-tweetChan:
			if !ok {
				keepGoing = false
			}
			tweetCount++
			globalUsers[newTweet.User.ScreenName]++
			globalUserTweets[newTweet.User.ScreenName] = append(globalUserTweets[newTweet.User.ScreenName], newTweet.Text)

		case <-checkTweetCount.C:
			go func() {
				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Total Number of tweets seen so far:")
				fmt.Println(tweetCount)
			}()

		case <-userTicker.C:
			timeElapsed += checkUserFrequency
			fmt.Printf("%d minutes of analysis\n", int(timeElapsed.Minutes()))
			countUser := make(map[int][]string)
			for user, count := range globalUsers {
				countUser[count] = append(countUser[count], user)
			}

			// everything after this should be ok to
			// run in a separate go routine i.e. unblock
			// the code above
			go func() {
				counts := getUserCounts(countUser, int(timeElapsed.Minutes()))
				printUserCounts(counts, countUser)
				hashTagCounts, mentionCounts := analyzeUserTweets(counts, countUser, globalUserTweets)

				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Hash tag counts")
				printSortedCount(hashTagCounts, int(timeElapsed.Minutes()))

				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Mention counts")
				printSortedCount(mentionCounts, int(timeElapsed.Minutes()))
			}()

		}

	}
}

func printStreamHeader() {
	fmt.Println("Commencing stream analysis")
	time.Sleep(time.Millisecond * 800)
	fmt.Println("...")
	time.Sleep(time.Millisecond * 800)
	fmt.Println("...")

}

func createTwitterStream(twitterClient *twitter.Client, streamParams []string) *twitter.Stream {
	fmt.Println(strings.Repeat("-", 90))
	fmt.Println("New stream tracking these parameters:")
	for _, param := range streamParams {
		fmt.Println(param)
	}
	fmt.Println(strings.Repeat("-", 90))

	formattedParams := []string{}
	for _, param := range streamParams {
		formattedParams = append(formattedParams, fmt.Sprintf("@%v", param))
		formattedParams = append(formattedParams, fmt.Sprintf("to:%v", param))
	}

	filterParams := &twitter.StreamFilterParams{
		Track:         formattedParams,
		StallWarnings: twitter.Bool(true),
	}
	stream, err := twitterClient.Streams.Filter(filterParams)
	if err != nil {
		log.Fatal(err)
	}

	return stream

}

func main() {

	streamParams := []string{
		"realDonaldTrump",
		"inittowinit007",
		"QAnon_",
		"QAnon",
	}

	// created buffered channel because userTicker will block
	// on tweet receive
	tweetChan := make(chan *twitter.Tweet, 5000)
	// stream will pass tweets to the tweetChan
	demux := createDemux(tweetChan)

	twitterClient := initApp()
	twitterStream := createTwitterStream(twitterClient, streamParams)
	printStreamHeader()
	go demux.HandleChan(twitterStream.Messages)

	// handle the tweets
	var wg sync.WaitGroup
	wg.Add(1)
	go handleTweets(&wg, tweetChan)
	go func() {
		wg.Wait()
	}()

	// ------------------  Wait for SIGINT and SIGTERM (HIT CTRL-C)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)

	fmt.Println("Stopping Stream...")
	twitterStream.Stop()
	// ---------------------------------------------------------------
}
