package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand"
	"sort"

	"github.com/expixel/actual-trivia-server/trivia"
)

// ErrMaxQuestionFetches is returned when too many trips have to be made to the database to retrieve questions.
var ErrMaxQuestionFetches = errors.New("maximum number of question fetches reached")

type questionService struct {
	db *sql.DB
}

func generateSortedRandom(buffer []int, min int, max int) {
	for i := 0; i < len(buffer); i++ {
		buffer[i] = rand.Intn(max-min+1) + min
	}
	sort.Ints(buffer)
}

func (s *questionService) GetQuestionCount() (int, error) {
	var questionCount int
	err := s.db.QueryRow(`SELECT count(*) FROM questions;`).Scan(&questionCount)
	if err != nil {
		return 0, err
	}
	return questionCount, nil
}

func (s *questionService) GetQuestionCounts() (minID int, maxID int, questionsCount int, err error) {
	err = s.db.QueryRow(`
		SELECT
			min(id) AS min,
			max(id) AS max,
			(
				SELECT reltuples AS ct
				FROM pg_class WHERE oid = 'questions'::regclass
			) AS count
		FROM questions;`).Scan(&minID, &maxID, &questionsCount)
	return
}

func (s *questionService) GetRandomQuestions(count int) ([]trivia.Question, error) {
	minID, maxID, _, err := s.GetQuestionCounts()
	if err != nil {
		return nil, err
	}
	maxQuestions := maxID - minID // we use the estimate instead

	// #TODO turn this into a prepared statement probably.
	// (count, questions_count, count + buffer, minID)
	statement := `
	WITH RECURSIVE random_pick AS (
		SELECT *
		FROM (
			SELECT $4 + trunc(random() * $2)::int as id
			FROM generate_series(1, $3)
			LIMIT $3									-- hint for query planner
		) r
		JOIN questions q USING(id)						-- eliminate misses
	
		UNION											-- eliminate dupes
		SELECT q.*
		FROM (
			SELECT $4 + trunc(random() * $2)::int AS id
			FROM random_pick r							-- just to make it recursive
			LIMIT $1									-- hint for query planner
		) r
	
		JOIN questions q USING (id)						-- eliminate misses
	)
	SELECT id, category, difficulty, prompt, choices, correct_choice, source
	FROM random_pick
	LIMIT $1;
	`

	// the maximum number of trips to the database for questions before we get an error.
	const maxFetches = 10

	fetchCount := 1
	questions := make(map[int64]trivia.Question)
	qcount := 0
	for qcount < count {
		retrieve := count - qcount
		if retrieve < 10 {
			retrieve = 10
		}

		retrievePlusBuffer := int(float64(retrieve) * 1.05) // +5%
		rows, err := s.db.Query(statement, retrieve, maxQuestions, retrievePlusBuffer, minID)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var choicesRaw string
			var q trivia.Question
			if err = rows.Scan(&q.ID, &q.Category, &q.Difficulty, &q.Prompt,
				&choicesRaw, &q.CorrectChoice, &q.Source); err != nil {
				return nil, err
			}

			q.Choices = make([]string, 0)
			json.Unmarshal([]byte(choicesRaw), &q.Choices)

			if _, ok := questions[q.ID]; !ok {
				questions[q.ID] = q
				qcount++
			}
		}

		rows.Close() // #TODO do I even check this error?

		fetchCount++
		if fetchCount > maxFetches {
			return nil, ErrMaxQuestionFetches
		}
	}

	questionsSlice := make([]trivia.Question, count)
	sliceIdx := 0
	for _, v := range questions {
		questionsSlice[sliceIdx] = v
		sliceIdx++

		// we might have more than we need.
		if sliceIdx > len(questionsSlice) {
			break
		}
	}
	return questionsSlice, nil
}

// NewQuestionService creates a new service for fetching questions from postgres.
func NewQuestionService(db *sql.DB) trivia.QuestionService {
	return &questionService{db: db}
}
