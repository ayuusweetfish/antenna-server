package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

////// Models //////

var db *sql.DB

type tableSchema struct {
	table   string
	columns []string
}

var schemata []tableSchema

func registerSchema(table string, columns ...string) {
	schemata = append(schemata, tableSchema{table, columns})
}

func InitializeSchemata() error {
	for _, schema := range schemata {
		var cmd strings.Builder
		cmd.WriteString("CREATE TABLE IF NOT EXISTS " + schema.table + " (")
		for i, columnDesc := range schema.columns {
			// columnName := strings.SplitN(columnDesc, " ", 2)[0]
			if i > 0 {
				cmd.WriteString(", ")
			}
			cmd.WriteString(columnDesc)
		}
		cmd.WriteString(")")
		cmdStr := cmd.String()
		if _, err := db.Exec(cmdStr); err != nil {
			return err
		}
	}
	return nil
}

func ConnectSQL() error {
	var err error

	db, err = sql.Open("sqlite3", "antenna.db")
	if err != nil {
		return err
	}

	return InitializeSchemata()
}

func ResetDatabase() error {
	for _, schema := range schemata {
		_, err := db.Exec("DROP TABLE IF EXISTS " + schema.table)
		if err != nil {
			return err
		}
	}
	return InitializeSchemata()
}

var rcli *redis.Client = nil

func ConnectRedis() {
	rcli = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

func nullIfZero(n interface{}) interface{} {
	if n == 0 {
		return nil
	} else {
		return n
	}
}

type DirectMarshal string

func (m DirectMarshal) MarshalJSON() ([]byte, error) {
	bytes := []byte(m)
	// Syntax errors will be reported by the `json` package
	/* if !json.Valid(bytes) {
		return nil, fmt.Errorf("Trying to send an invalid valid JSON encoding verbatim")
	} */
	return bytes, nil
}

type OrderedKeysMarshal []struct {
	key   string
	value interface{}
}

func (m OrderedKeysMarshal) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteRune('{')
	for i, entry := range m {
		if i > 0 {
			buf.WriteRune(',')
		}
		k, err := json.Marshal(entry.key)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteRune(':')
		v, err := json.Marshal(entry.value)
		if err != nil {
			return nil, err
		}
		buf.Write(v)
	}
	buf.WriteRune('}')
	return buf.Bytes(), nil
}

////// Models //////

type User struct {
	Id       int
	Nickname string
	Email    string
	Password string
}

func init() {
	registerSchema("user",
		"id INTEGER UNIQUE PRIMARY KEY AUTOINCREMENT",
		"nickname TEXT",
		"email TEXT",
		"password TEXT")
}

func (u *User) Repr() map[string]interface{} {
	return map[string]interface{}{
		"id":       u.Id,
		"nickname": u.Nickname,
	}
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
		"SELECT nickname, email, password FROM user WHERE id = $1", u.Id,
	).Scan(&u.Nickname, &u.Email, &u.Password)
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
	err := db.QueryRow("INSERT OR REPLACE INTO user(id, nickname, email, password) "+
		"VALUES($1, $2, $3, $4) RETURNING id",
		nullIfZero(u.Id), u.Nickname, u.Email, u.Password,
	).Scan(&u.Id)
	if err != nil {
		panic(err)
	}
}

type Profile struct {
	Id      int
	Creator int
	Details string
	Stats   [8]int
	Traits  []string
}

func init() {
	registerSchema("profile",
		"id INTEGER UNIQUE PRIMARY KEY AUTOINCREMENT",
		"creator INTEGER",
		"details TEXT",
		"stats TEXT",
		"traits TEXT",
		"FOREIGN KEY (creator) REFERENCES user(id)")
}

func (p *Profile) Repr() OrderedKeysMarshal {
	return OrderedKeysMarshal{
		{"id", p.Id},
		{"details", DirectMarshal(p.Details)},
		{"stats", p.Stats},
		{"traits", p.Traits},
	}
}

func parseProfileStats(s string) ([8]int, error) {
	stats := strings.Split(s, ",")
	if len(stats) != 8 {
		return [8]int{}, fmt.Errorf("Stats should be of length 8")
	}

	var statsN [8]int
	for i := range 8 {
		val, err := strconv.ParseUint(stats[i], 10, 8)
		if err != nil || val < 10 || val > 90 {
			return [8]int{}, fmt.Errorf("Incorrect stat value \"%s\"", stats[i])
		}
		statsN[i] = int(val)
	}
	return statsN, nil
}
func encodeProfileStats(stats [8]int) string {
	var builder strings.Builder
	for i := range 8 {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(strconv.Itoa(int(stats[i])))
	}
	return builder.String()
}

func parseProfileTraits(s string) []string {
	return strings.Split(s, ",")
}
func encodeProfileTraits(traits []string) string {
	return strings.Join(traits, ",")
}

func (p *Profile) Save() {
	err := db.QueryRow("INSERT OR REPLACE INTO profile(id, creator, details, stats, traits) "+
		"VALUES($1, $2, $3, $4, $5) RETURNING id",
		nullIfZero(p.Id), p.Creator, p.Details,
		encodeProfileStats(p.Stats), encodeProfileTraits(p.Traits),
	).Scan(&p.Id)
	if err != nil {
		panic(err)
	}
}

func (p *Profile) Load() bool {
	var stats, traits string
	err := db.QueryRow(
		"SELECT creator, details, stats, traits FROM profile WHERE id = $1",
		p.Id,
	).Scan(
		&p.Creator,
		&p.Details,
		&stats,
		&traits,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		panic(err)
	}
	if p.Stats, err = parseProfileStats(stats); err != nil {
		panic(err)
	}
	p.Traits = parseProfileTraits(traits)
	return true
}

type Room struct {
	Id          string
	Title       string
	Tags        string
	Description string
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
