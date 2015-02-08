package tweet_words

import (
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"strings"
)

type User struct {
	Token    string
	Secret   string
	Keywords []string
}

// TODO (vin18) Currently only one global user is tracked
// i.e the last logged in user.
// Remove dependency on GUser
var GUser User

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func StoreUser(user User) (ret bool) {
	mgoSession, err := mgo.Dial(Conf["MONGO"])
	if err != nil {
		panic(err)
	}
	mgoSession.SetMode(mgo.Monotonic, true)
	defer mgoSession.Close()
	newSes := mgoSession.Copy()
	defer newSes.Close()
	db := newSes.DB("test")
	r, _ := db.CollectionNames()
	fmt.Println(r)
	col := db.C("User")
	if col == nil {
		panic("unable to get collection")
	}
	result := User{}
	fmt.Println(bson.M{"token": user.Token})
	err = col.Find(bson.M{"token": user.Token}).One(&result)
	if result.Token == "" {
		dummy := []string{"bjp"}
		err = col.Insert(&User{user.Token, user.Secret, dummy})
		if err != nil {
			panic(err)
		}
	}
	err = col.Find(bson.M{"token": user.Token}).One(&result)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func StoreKeywords(data string) (ret bool) {
	//convert to lower case
	data = strings.ToLower(data)

	mgoSession, err := mgo.Dial(Conf["MONGO"])
	if err != nil {
		panic(err)
	}
	mgoSession.SetMode(mgo.Monotonic, true)
	defer mgoSession.Close()
	newSes := mgoSession.Copy()
	defer newSes.Close()
	col := newSes.DB("test").C("User")
	if col == nil {
		panic("unable to get collection")
	}
	result := User{}
	err = col.Find(bson.M{"token": GUser.Token}).One(&result)
	if err != nil {
		log.Fatal(err)
	}

	// Insert only unique entries as Keywords
	if !stringInSlice(data, result.Keywords) {
		n := len(result.Keywords)
		words := make([]string, n+1)
		copy(words, result.Keywords[0:])
		words[n] = data
		_, err = col.Upsert(bson.M{"token": GUser.Token, "secret": GUser.Secret}, bson.M{"$set": bson.M{"keywords": words}})
	}

	if err != nil {
		fmt.Println(err)
	}

	result1 := User{}
	err = col.Find(bson.M{"token": GUser.Token}).One(&result1)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func GetKeywords() (ret []string) {
	mgoSession, err := mgo.Dial(Conf["MONGO"])
	if err != nil {
		panic(err)
	}
	mgoSession.SetMode(mgo.Monotonic, true)
	defer mgoSession.Close()
	newSes := mgoSession.Copy()
	defer newSes.Close()
	col := newSes.DB("test").C("User")
	if col == nil {
		panic("unable to get collection")
	}
	result := User{}
	err = col.Find(bson.M{"token": GUser.Token}).One(&result)
	if err != nil {
		log.Fatal(err)
	}
	return result.Keywords
}

func GetTweets(keyValue string) (retValue []TweetStore) {
	mgoSession, err := mgo.Dial(Conf["MONGO"])
	if err != nil {
		panic(err)
	}
	mgoSession.SetMode(mgo.Monotonic, true)
	defer mgoSession.Close()
	newSes := mgoSession.Copy()
	defer newSes.Close()
	col := newSes.DB("test").C(keyValue)
	if col == nil {
		panic("unable to get collection")
	}
	result := []TweetStore{}
	err = col.Find(bson.M{}).All(&result)
	if err != nil {
		log.Fatal(err)
	}
	return result
}
