package main

import (
	"log"
	"os"

	"github.com/dghubble/oauth1"
)

func GetTokens() (*oauth1.Token, *oauth1.Config) {
	consumerKey := os.Getenv("consumer_key")
	consumerSecret := os.Getenv("consumer_secret")
	accessToken := os.Getenv("access_token")
	accessSecret := os.Getenv("access_token_secret")

	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	return token, config
}
