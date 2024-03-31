package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	GenderFemale = iota
	GenderMale
	GenderNonBinary
)

type ProfileDetails struct {
	Race       string
	Age        string
	Occupation string
	Likes      string
	Dislikes   string
	Background string
}
type Profile struct {
	Avatar      string
	Gender      int8
	Orientation int8 // bitmask
	Details     ProfileDetails
	Stats       [8]int8
	Traits      []string
}

type Room struct {
	Id          string
	Title       string
	Tags        string
	Description string
}

var rcli *redis.Client = nil

func ConnectRedis() {
	rcli = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

func (r *Room) Load() error {
	val, err := rcli.HGetAll(context.Background(), "room:"+r.Id).Result()
	if err != nil {
		return err
	}
	r.Title = val["title"]
	r.Tags = val["tags"]
	r.Description = val["description"]
	return nil
}

func (r *Room) Save() error {
	val, err := rcli.Incr(context.Background(), "room_count").Result()
	if err != nil {
		return err
	}
	r.Id = fmt.Sprintf("%d", val)
	_, err = rcli.HSet(context.Background(), "room:"+r.Id,
		"title", r.Title,
		"tags", r.Tags,
		"description", r.Description).Result()
	return err
}
