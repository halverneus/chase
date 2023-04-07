package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const year = 2022

const (
	credit = "CREDIT"
	debit  = "DEBIT"
	dslip  = "DSLIP"
	check  = "CHECK"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("Expected 1 argument: source PDF file")
	}
	file := os.Args[1]

	html, err := convertToHTML(file)
	if err != nil {
		log.Fatalln(err)
	}

	if err := run(html); err != nil {
		for _, l := range entries {
			fmt.Print(l)
		}
		log.Fatalln(err)
	}

	// fmt.Println("Details,Posting Date,Description,Amount,Type,Balance,Check or Slip #")
	// for _, l := range entries {
	// 	fmt.Print(l)
	// }
	// Write to file.
	f, err := os.Create("output.csv")
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	// f.WriteString("Details,Posting Date,Description,Amount,Type,Balance,Check or Slip #\n")
	for _, l := range entries {
		f.WriteString(l.String())
	}
}

func convertToHTML(filename string) (string, error) {
	cmd := exec.Command("pdftohtml", "-stdout", filename)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type state uint8

const (
	ignore state = iota
	transactionHeader
	nextDate
	nextName
	nextAmount
	nextTotal
)

func getDate(line string) (string, error) {
	if !strings.HasSuffix(line, "<br/>") {
		return "", fmt.Errorf("getDate: line '%s' does not end with <br/>", line)
	}
	line = strings.TrimSuffix(line, "<br/>")
	parts := strings.Split(line, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("getDate: line '%s' does not contain two parts split by '/'", line)
	}
	month, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("getDate: line '%s' does not contain a valid month", line)
	}
	day, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("getDate: line '%s' does not contain a valid day", line)
	}

	// Return a string formatted mm/dd/yyyy
	return fmt.Sprintf("%02d/%02d/%d", month, day, year), nil
}

func getDetail(line string) (string, error) {
	if !strings.HasSuffix(line, "<br/>") {
		return "", fmt.Errorf("getDetail: line '%s' does not end with <br/>", line)
	}
	line = strings.TrimSuffix(line, "<br/>")
	line = strings.ReplaceAll(line, "&#160;", " ")

	// Remove duplicate spaces.
	line = strings.Join(strings.Fields(line), " ")
	return line, nil
}

func isDetail(line string) bool {
	if strings.Contains(line, "&#160;") {
		return true
	}
	if strings.HasPrefix(line, "<b>") {
		line = strings.TrimPrefix(line, "<b>")
	}
	if strings.HasSuffix(line, "<br/>") {
		line = strings.TrimSuffix(line, "<br/>")
	}
	if strings.HasSuffix(line, "</b>") {
		line = strings.TrimSuffix(line, "</b>")
	}
	if strings.Contains(line, ",") {
		return false
	}
	val, err := strconv.ParseFloat(line, 64)
	if err != nil {
		return true
	}
	if val > 100000 {
		return true
	}
	v := line[0]
	if v == '-' {
		return false
	}
	return v < '0' || v > '9'
}

func getAmount(line string) (ttype, amount string, err error) {
	if !strings.HasSuffix(line, "<br/>") {
		return "", "", fmt.Errorf("getAmount: line '%s' does not end with <br/>", line)
	}
	line = strings.TrimSuffix(line, "<br/>")

	if strings.HasPrefix(line, "<b>") {
		line = strings.TrimPrefix(line, "<b>")
		line = strings.TrimSuffix(line, "</b>")
	}

	if strings.HasPrefix(line, "-") {
		ttype = debit
	} else {
		ttype = credit
	}
	line = strings.ReplaceAll(line, ",", "")
	return ttype, line, nil
}

func getTotal(line string) (total string, err error) {
	if !strings.HasSuffix(line, "<br/>") {
		return "", fmt.Errorf("getTotal: line '%s' does not end with <br/>", line)
	}
	line = strings.TrimSuffix(line, "<br/>")
	line = strings.ReplaceAll(line, ",", "")
	return line, nil
}

type entry struct {
	date   string
	detail string
	amount string
	ttype  string
	total  string
}

func (e *entry) String() string {
	return fmt.Sprintf(
		"%s,%s,\"%s\",%s,ACH_%s,%s,,\n",
		e.ttype, e.date, e.detail, e.amount, e.ttype, e.total,
	)
}

var entries []*entry

func run(html string) error {
	currentState := ignore
	var e *entry
	var err error

	lines := strings.Split(html, "\n")
	for i, line := range lines {
		err = func() error {
			switch currentState {
			case ignore:
				if strings.Contains(line, "*start*transactiondetail<br/>") {
					currentState = transactionHeader
				}
				return nil

			case transactionHeader:
				if strings.HasPrefix(line, "<b>") {
					return nil
				}
				var date string
				if date, err = getDate(line); err != nil {
					return err
				}
				e = &entry{date: date}
				currentState = nextName
				return nil

			case nextDate:
				if strings.Contains(line, "*end*transaction&#160;detail<br/>") {
					currentState = ignore
					return nil
				}
				if strings.Contains(line, "<b>Ending&#160;Balance</b><br/>") {
					currentState = ignore
					return nil
				}
				var date string
				if date, err = getDate(line); err != nil {
					detail, err := getDetail(line)
					if err != nil {
						return err
					}
					e.detail = fmt.Sprintf("%s %s", e.detail, detail)
					fmt.Printf("WARNING: Wrapped line: %s\n", line)
					return nil
				}
				e = &entry{date: date}
				currentState = nextName
				return nil

			case nextName:
				var part string
				if part, err = getDetail(line); err != nil {
					return err
				}
				if len(e.detail) == 0 {
					e.detail = part
				} else {
					e.detail = fmt.Sprintf("%s %s", e.detail, part)
				}
				nextLine := lines[i+1]
				if strings.Contains(part, "Remote Online Deposit") && strings.Contains(nextLine, "1<br/>") {
					return nil
				}
				if !isDetail(nextLine) {
					currentState = nextAmount
				}
				return nil

			case nextAmount:
				if e.ttype, e.amount, err = getAmount(line); err != nil {
					return err
				}
				currentState = nextTotal
				return nil

			case nextTotal:
				if e.total, err = getTotal(line); err != nil {
					return err
				}
				currentState = nextDate
				entries = append(entries, e)

				return nil

			default:
				return fmt.Errorf("Unknown state: %d", currentState)
			}
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
