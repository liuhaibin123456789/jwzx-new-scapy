package tool

import (
	"github.com/go-redis/redis"
	"log"
	"strconv"
	"time"
)

var RDB = &redis.Client{}

func LinkRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

//SetUrl 保存一天
func SetUrl(key int, url string) error {
	err := RDB.Set(strconv.Itoa(key), url, time.Hour*24).Err()
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func GetUrl(key int) (string, error) {
	get := RDB.Get(strconv.Itoa(key))
	if err := get.Err(); err != nil {
		return "", err
	}
	return get.String(), nil
}
