package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Id       int
	Nickname string
	Email    string
	Password string
}

const (
	GenderFemale = iota
	GenderMale
	GenderNonBinary
)

type Profile struct {
	Creator     int
	Avatar      string
	Gender      int8
	Orientation int8 // bitmask
	Details     string
	Stats       [8]int8
	Traits      []string
}

type Room struct {
	Id          string
	Title       string
	Tags        string
	Description string
}

var db *sql.DB

func ConnectSQL() error {
	var err error

	db, err = sql.Open("sqlite3", "antenna.db")
	if err != nil {
		return err
	}

	cmd := "CREATE TABLE IF NOT EXISTS players" +
		"(id INTEGER UNIQUE PRIMARY KEY AUTOINCREMENT, nickname TEXT, email TEXT, password TEXT)"
	if _, err := db.Exec(cmd); err != nil {
		db.Close()
		return err
	}

	return nil
}

var rcli *redis.Client = nil

func ConnectRedis() {
	rcli = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

func (u *User) hashPassword() {
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	u.Password = string(hashed)
}

func (u *User) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func (u *User) LoadById() bool {
	err := db.QueryRow(
		"SELECT nickname, email, password FROM players WHERE id = $1", u.Id).
		Scan(&u.Nickname, &u.Email, &u.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		panic(err)
	}
	return true
}

func (u *User) Save() {
	u.hashPassword()
	err := db.QueryRow("INSERT INTO players(nickname, email, password) "+
		"VALUES($1, $2, $3) "+
		"ON CONFLICT(id) DO UPDATE SET "+
		"nickname = excluded.nickname, "+
		"email = excluded.email, "+
		"password = excluded.password"+
		" RETURNING id",
		u.Nickname, u.Email, u.Password).
		Scan(&u.Id)
	if err != nil {
		panic(err)
	}
}

func (r *Room) Load() {
	val, err := rcli.HGetAll(context.Background(), "room:"+r.Id).Result()
	if err != nil {
		panic(err)
	}
	r.Title = val["title"]
	r.Tags = val["tags"]
	r.Description = val["description"]
}

func (r *Room) Save() {
	val, err := rcli.Incr(context.Background(), "room_count").Result()
	if err != nil {
		panic(err)
	}
	r.Id = fmt.Sprintf("%d", val)
	_, err = rcli.HSet(context.Background(), "room:"+r.Id,
		"title", r.Title,
		"tags", r.Tags,
		"description", r.Description).Result()
}
