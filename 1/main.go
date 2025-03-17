package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Quiz struct {
	Question string
	Answer   string
}

type TimeoutAnswer struct {
	answer []byte
	err    error
}

func readRecords(ctx context.Context, filename string) <-chan []string {
	records := make(chan []string)

	go func() {
		defer close(records)

		fileHandler, err := os.Open(filename)

		if err != nil {
			log.Fatal(err)
		}

		defer fileHandler.Close()

		reader := csv.NewReader(fileHandler)

		for {
			record, err := reader.Read()

			if record == nil && err == io.EOF {
				return
			}

			if err != nil {
				log.Fatalln(err)
			}

			select {
			case <-ctx.Done():
				return
			case records <- record:
			}
		}
	}()

	return records
}

func parseRecords(ctx context.Context, records <-chan []string) <-chan Quiz {
	quizzes := make(chan Quiz)

	go func() {
		defer close(quizzes)

		for record := range records {
			quiz := Quiz{
				Question: record[0],
				Answer:   strings.ToLower(strings.TrimSpace(record[1])),
			}

			select {
			case <-ctx.Done():
				return
			case quizzes <- quiz:
			}
		}
	}()

	return quizzes
}

func startQuiz(signalCtx context.Context, quizzes <-chan Quiz, answerTimeLimit int) {
	correct, wrong, total := 0, 0, 0

	answers := make(chan TimeoutAnswer)

	var userAnswer TimeoutAnswer
	timeout := false
	timerLimit := time.Duration(answerTimeLimit) * time.Second

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("You have %s seconds to answer each question.\nIf you exceed the timeout then it is game over.\nAre you ready?\npress any key to start the quiz.\n", timerLimit)

	if _, err := reader.ReadBytes('\n'); err != nil {
		log.Fatalln("Failed to read line due error: ", err)
	}

	readAnswer := func(readContext context.Context) {
		fmt.Print("Answer: ")
		userAnswer, err := reader.ReadBytes('\n')
		answer := TimeoutAnswer{userAnswer, err}

		select {
		case <-signalCtx.Done():
			log.Println("SIGNAL")
			return
		case <-readContext.Done():
			log.Println("CANCELLED")
			return
		case answers <- answer:
		}
	}

	readContext, readContextCancel := context.WithCancel(context.Background())
	defer readContextCancel()

	for quiz := range quizzes {
		fmt.Printf("Question %d: %s?\n", total+1, quiz.Question)

		go readAnswer(readContext)

		select {
		case <-time.After(timerLimit):
			timeout = true
		case <-signalCtx.Done():
			return
		case answer := <-answers:
			userAnswer = answer
		}

		if timeout {
			fmt.Println("\nTimeout")
			break
		}

		if userAnswer.err != nil {
			log.Printf("Error in answering the question %s due %v\n", quiz.Question, userAnswer.err.Error())
			continue
		}

		userAnswerString := string(bytes.ToLower(bytes.TrimSpace(userAnswer.answer)))

		if userAnswerString == quiz.Answer {
			correct++
		} else {
			wrong++
		}

		total++
	}

	var correctAnswersPercentage float32
	var wrongAnswersPercentage float32

	if total > 0 {
		correctAnswersPercentage = (float32(correct) / float32(total)) * 100
		wrongAnswersPercentage = (float32(wrong) / float32(total)) * 100
	}

	fmt.Printf("Total: %d | Correct: %d | Wrong: %d | Correct ratio: %.2f%% | Wrong ratio: %.2f%%\n", total, correct, wrong, correctAnswersPercentage, wrongAnswersPercentage)
}

func main() {
	quizTimeout := flag.Int("timeout", 30, "Time limit to answer a question.")
	filePath := flag.String("file", "problems.csv", "quiz csv filepath")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	records := readRecords(ctx, *filePath)
	quizzes := parseRecords(ctx, records)

	startQuiz(ctx, quizzes, *quizTimeout)
}
