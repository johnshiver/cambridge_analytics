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

var token *oauth1.Token
var config *oauth1.Config

func getTokens() {
	consumerKey := os.Getenv("consumer_key")
	consumerSecret := os.Getenv("consumer_secret")
	accessToken := os.Getenv("access_token")
	accessSecret := os.Getenv("access_token_secret")

	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

	config = oauth1.NewConfig(consumerKey, consumerSecret)
	token = oauth1.NewToken(accessToken, accessSecret)
}

func inspectHashTagsFromUserTweets(userTweets []string) []string {
	hash_tags := []string{}
	for _, tweet := range userTweets {
		words := strings.Split(tweet, " ")
		for _, word := range words {
			if strings.HasPrefix(word, "#") {
				hash_tags = append(hash_tags, word)
			}
		}
	}
	return hash_tags

}

func main() {
	//	possible_pro_russia_trolls := []string{
	//		"UniteAlbertans2",
	//		"KAGcommittee",
	//		"CovfefeNation",
	//		"PISTOLGRIP6spd",
	//		"boriskogan5",
	//		"avonsalez",
	//		"MarthaLimKhemra",
	//	}
	//	possible_pro_resistance_trolls := []string{
	//		"rjakes65",
	//	}
	//
	getTokens()
	auth_http_client := config.Client(oauth1.NoContext, token)
	twitter_client := twitter.NewClient(auth_http_client)

	tweet_chan := make(chan *twitter.Tweet, 5000)
	done := make(chan interface{})
	demux := createDemux(tweet_chan)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		globalUsers := make(map[string]int, 0)
		globalUserTweets := make(map[string][]string)
		globalHashTagCounts := make(map[string]int, 0)
		minutes := 0
		user_ticker := time.NewTicker(time.Minute * 1)
		for {
			select {
			case new_tweet, ok := <-tweet_chan:
				if !ok {
					done <- struct{}{}
				}
				globalUsers[new_tweet.User.ScreenName] += 1
				globalUserTweets[new_tweet.User.ScreenName] = append(globalUserTweets[new_tweet.User.ScreenName], new_tweet.Text)

			case <-user_ticker.C:
				minutes += 1
				fmt.Printf("%d minutes of analysis\n", minutes)
				countUser := make(map[int][]string)
				for user, count := range globalUsers {
					countUser[count] = append(countUser[count], user)
				}

				go func() {
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
					sort.Ints(counts)
					for _, i := range counts {
						fmt.Println(strings.Repeat("-", 45))
						fmt.Println(i)
						for _, u := range countUser[i] {
							fmt.Println(u)
							fmt.Println(strings.Repeat("-", 45))
							user_tweets := globalUserTweets[u]
							hash_tags := inspectHashTagsFromUserTweets(user_tweets)
							for _, hash_tag := range hash_tags {
								globalHashTagCounts[hash_tag] = 1
							}

						}
						for hash_tag, _ := range globalHashTagCounts {
							fmt.Println(hash_tag)
						}
					}

					fmt.Println(strings.Repeat("-", 45))
				}()

			case <-done:
				return
			default:
				continue
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
