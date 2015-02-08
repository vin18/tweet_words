package main

import (
		"github.com/mukundmr/tweet_words"
		"net/http"
		"flag"
		"log"
		)

func main() {
	flag.Parse()
	tweet_words.ReadCredentials();

	http.Handle("/bower_components/", http.StripPrefix("/bower_components/", http.FileServer(http.Dir("./bower_components/"))))
	http.Handle("/elements/", http.StripPrefix("/elements/", http.FileServer(http.Dir("./elements/"))))
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("./img/"))))

	// Use a different auth URL for "Sign in with Twitter."
	tweet_words.SigninOAuthClient = tweet_words.OauthClient
	tweet_words.SigninOAuthClient.ResourceOwnerAuthorizationURI = "https://api.twitter.com/oauth/authenticate"

	tweet_words.Home()
	http.HandleFunc("/signin", tweet_words.ServeSignin)
	http.HandleFunc("/logout", tweet_words.ServeLogout)
	http.HandleFunc("/callback", tweet_words.ServeOAuthCallback)
	http.HandleFunc("/storeKeyword", tweet_words.StoreKeywordServ)
	http.HandleFunc("/GetKeywordsServ", tweet_words.GetKeywordsServ)
	if err := http.ListenAndServe(*tweet_words.HttpAddr, nil); err != nil {
		log.Fatalf("Error listening, %v", err)
	}
}

