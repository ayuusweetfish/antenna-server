package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Lofanmi/pinyin-golang/pinyin"
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

func commaSep(values []int) string {
	var b strings.Builder
	for i, v := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(v))
	}
	return b.String()
}

func main() {
	defer func() {
		if obj := recover(); obj != nil {
			fmt.Fprintln(os.Stderr, obj)
		}
	}()

	dict := pinyin.NewDict()
	s := dict.Convert(`浪漫反应`, " ").ASCII()
	fmt.Println(s)
	// return

	// Open file
	f, err := excelize.OpenFile("1.xlsx")
	panicIf(err)
	defer func() {
		panicIf(f.Close())
	}()

	activeSheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(activeSheet)
	panicIf(err)

	occurrenceRowIndex := make(map[string]int) // For deduplication
	for i, row := range rows {
		card, err := tryParseRow(row)
		if err == nil {
			// fmt.Println(card)
			// Check for duplication
			if j, ok := occurrenceRowIndex[card.Name]; ok {
				fmt.Fprintf(os.Stderr, "Skipped row %d (%s duplicate at row %d)\n",
					i, card.Name, j)
				continue
			}
			occurrenceRowIndex[card.Name] = i
			fmt.Printf(`"%s": {[]int{%s}, %d, [3]int{%s}},`+"\n",
				card.Name, commaSep(card.Condition), card.Growth, commaSep(card.RelationshipChange[:]))
			// fmt.Printf(`"%s": [[%s], %d, [%s]],`+"\n",
			// 	card.Name, commaSep(card.Condition), card.Growth, commaSep(card.RelationshipChange[:]))
		} else {
			fmt.Fprintf(os.Stderr, "Skipped row %d (%v)\n", i+1, err)
		}
	}
}
