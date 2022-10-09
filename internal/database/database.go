package database

import (
	"github.com/go-redis/redis"
)

var client *redis.Client

//set new redis client and check that it works
func New() error {
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       5,
	})

	_, err := client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func Set(key string, value any) {
	client.Set(key, value, 0)
}

func HSet(name, key, value string) {
	client.HSet(name, key, value)
}

func ZAdd(key string, value redis.Z) {
	client.ZAdd(key, value)
}

func Incr(key string) {
	client.Incr(key)
}
func Del(key string) {
	client.Del(key)
}

func ZRevRangeByScore(key string, op redis.ZRangeBy) ([]string, error) {
	data, err := client.ZRevRangeByScore(key, op).Result()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func HGet(name, key string) (string, error) {
	data, err := client.HGet(name, key).Result()
	if err != nil {
		return "", err
	}
	return data, nil
}
