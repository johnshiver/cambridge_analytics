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

func createDemux(tweet_chan chan *twitter.Tweet) *twitter.SwitchDemux {
	demux := twitter.NewSwitchDemux()
	demux.Tweet = func(tweet *twitter.Tweet) {
		tweet_chan <- tweet
	}
	return &demux
}

func getLocations(twitter_client *twitter.Client) {
	locations, _, err := twitter_client.Trends.Available()
	if err != nil {
		log.Fatal(err)
	}
	if len(locations) > 0 {
		for _, location := range locations {
			spew.Dump(location)
		}
	}
}

func getTokens() (*oauth1.Token, *oauth1.Config) {
	consumerKey := os.Getenv("consumer_key")
	consumerSecret := os.Getenv("consumer_secret")
	accessToken := os.Getenv("access_token")
	accessSecret := os.Getenv("access_token_secret")

	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

	config = oauth1.NewConfig(consumerKey, consumerSecret)
	token = oauth1.NewToken(accessToken, accessSecret)
	return token, config
}

func extractFromUserTweets(userTweets []string) ([]string, []string) {
	hash_tags := []string{}
	mentions := []string{}
	for _, tweet := range userTweets {
		words := strings.Split(tweet, " ")
		for _, word := range words {
			if strings.HasPrefix(word, "#") {
				hash_tags = append(hash_tags, word)
			}
			if strings.HasPrefix(word, "@") {
				mentions = append(mentions, word)
			}
		}
	}
	return hash_tags, mentions

}

func printSortedCount(itemCounts map[string]int, threshold int) {
	counts := []int{}
	countItems := make(map[int][]string)
	for item, count := range itemCounts {
		countItems[count] = append(countItems[count], item)
	}

	for count, _ := range countItems {
		counts = append(counts, count)
	}

	sort.Ints(counts)
	for _, i := range counts {
		fmt.Println(strings.Repeat("-", 45))
		fmt.Println(i)
		fmt.Println(strings.Repeat("-", 45))
		for _, item := range countItems[i] {
			fmt.Println(item)
		}
	}
}

func initApp() *twitter.Client {

	token, config := getTokens()
	auth_http_client := config.Client(oauth1.NoContext, token)
	twitter_client := twitter.NewClient(auth_http_client)
	return twitter_client

}

func main() {
	twitter_client := initApp()

	// created buffered channel because user_ticker will block
	// on tweet receive
	tweet_chan := make(chan *twitter.Tweet, 5000)
	done := make(chan interface{})
	demux := createDemux(tweet_chan)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// probably bad design, but these are datastructures
		// that are shared between cases.
		// TODO use sync.Map for these...mb can take advantage
		// of more goroutines
		globalUsers := make(map[string]int, 0)
		globalUserTweets := make(map[string][]string)
		minutes := 0
		tweet_count := 0
		user_ticker := time.NewTicker(time.Minute * 5)
		check_tweet_count := time.NewTicker(time.Minute * 1)
		for {
			select {
			case new_tweet, ok := <-tweet_chan:
				if !ok {
					done <- struct{}{}
				}
				tweet_count += 1
				globalUsers[new_tweet.User.ScreenName] += 1
				globalUserTweets[new_tweet.User.ScreenName] = append(globalUserTweets[new_tweet.User.ScreenName], new_tweet.Text)

			case <-check_tweet_count.C:
				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Checking # of tweets")
				fmt.Println(tweet_count)

			case <-user_ticker.C:
				hashTagCounts := make(map[string]int, 0)
				mentionCounts := make(map[string]int, 0)
				minutes += 5
				fmt.Printf("%d minutes of analysis\n", minutes)
				countUser := make(map[int][]string)
				for user, count := range globalUsers {
					countUser[count] = append(countUser[count], user)
				}

				// everything after this should be ok to
				// run in a separate go routine i.e. unblock
				// the code above

				// turn this into function

				importantCounts := []map[int][]string{}
				counts := []int{}
				tempMap := make(map[int][]string)
				for count, users := range countUser {
					if count > minutes/2 && count > 1 {
						counts = append(counts, count)
						tempMap[count] = users
						importantCounts = append(importantCounts, tempMap)
					}
				}

				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("User counts")

				sort.Ints(counts)
				for _, i := range counts {
					fmt.Println(strings.Repeat("-", 45))
					fmt.Println(i)
					fmt.Println(strings.Repeat("-", 45))
					for _, u := range countUser[i] {
						fmt.Println(u)
						user_tweets := globalUserTweets[u]
						hash_tags, mentions := extractFromUserTweets(user_tweets)
						for _, hash_tag := range hash_tags {
							hashTagCounts[hash_tag] += 1
						}
						for _, mention := range mentions {
							mentionCounts[mention] += 1

						}

					}

				}
				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Global hash tag counts")
				printSortedCount(hashTagCounts, minutes)
				fmt.Println(strings.Repeat("#", 90))
				fmt.Println("Global mention counts")
				printSortedCount(mentionCounts, minutes)

			case <-done:
				return
				//default:
				//	continue
			}
		}

	}()

	go func() {
		wg.Wait()
	}()

	filterParams := &twitter.StreamFilterParams{
		Track:         []string{"to:realDonaldTrump", "@realDonaldTrump"},
		StallWarnings: twitter.Bool(true),
	}
	stream, err := twitter_client.Streams.Filter(filterParams)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Commencing stream analysis")
	time.Sleep(time.Millisecond * 800)
	fmt.Println("...")
	time.Sleep(time.Millisecond * 800)
	fmt.Println("...")

	go demux.HandleChan(stream.Messages)

	// Wait for SIGINT and SIGTERM (HIT CTRL-C)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)

	fmt.Println("Stopping Stream...")
	stream.Stop()
}
