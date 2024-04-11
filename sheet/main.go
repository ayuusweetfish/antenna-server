package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

type Card struct {
	Name               string
	Condition          []int
	Growth             int
	RelationshipChange [3]int
}

func tryParseRow(row []string) (Card, error) {
	card := Card{}
	var err error

	// Pad to length 7
	for len(row) < 7 {
		row = append(row, "")
	}

	card.Name = row[0]

	// Condition list
	card.Condition = []int{}
	state := -1
	for _, c := range row[2] {
		switch c {
		case 'S':
			state = 0
		case 'N':
			state = 1
		case 'T':
			state = 2
		case 'F':
			state = 3

		case 'i':
			fallthrough
		case 'e':
			if state != -1 {
				condIndex := state * 2
				if c == 'i' {
					condIndex += 1
				}
				card.Condition = append(card.Condition, condIndex)
				state = -1
			}

		default:
			state = -1
		}
	}

	// Numerical values
	card.Growth, err = strconv.Atoi(row[3])
	if err != nil {
		return card, err
	}
	for i := range 3 {
		s := row[4+i]
		if s == "" {
			card.RelationshipChange[i] = 0
		} else {
			card.RelationshipChange[i], err = strconv.Atoi(s)
			if err != nil {
				return card, err
			}
		}
	}
	return card, nil
}

func main() {
	defer func() {
		if obj := recover(); obj != nil {
			fmt.Fprintln(os.Stderr, obj)
		}
	}()

	// Open file
	f, err := excelize.OpenFile("1.xlsx")
	panicIf(err)
	defer func() {
		panicIf(f.Close())
	}()

	activeSheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(activeSheet)
	panicIf(err)

	for i, row := range rows {
		card, err := tryParseRow(row)
		if err == nil {
			fmt.Println(card)
		} else {
			fmt.Fprintf(os.Stderr, "Skipped row %d (%v)\n", i+1, err)
		}
	}
}
