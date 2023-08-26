package db

import (
	"context"
	"database/sql"

	"AlainDebot/log"

	"github.com/jmhodges/clock"
)

var (
	db  *sql.DB
	clk = clock.New()
)

var txIsoRepeatableRead = &sql.TxOptions{Isolation: sql.LevelRepeatableRead}

func Init(connStr string) {
	d, err := sql.Open("pgx", connStr)
	if err != nil {
		panic(err)
	}

	if err = d.Ping(); err != nil {
		panic(err)
	}

	log.Info("Successfully initialized database")

	db = d
}

func AddUser(usr, cht int64) error {
	tx, err := db.BeginTx(context.Background(), txIsoRepeatableRead)
	if err != nil {
		log.Error(usr, err, "failed starting transaction on adding user")
		return err
	}
	defer tx.Rollback()

	var cID int64
	err = tx.QueryRow(`SELECT chat_id FROM users WHERE id=$1`, usr).Scan(&cID)
	switch {
	case err == sql.ErrNoRows:
		query := `INSERT INTO users (id, chat_id, created_on) VALUES ($1, $2, $3)`
		if _, err = tx.Exec(query, usr, cht, clk.Now().UTC()); err != nil {
			log.Error(usr, err, "failed adding user")
			return err
		}

	case err != nil:
		log.Error(usr, err, "failed fetching chat ID")
		return err

	default:
		if cID == cht {
			log.Infof("user is already up-to-date")
			return nil
		}

		if _, err = tx.Exec(`UPDATE users SET chat_id=$1`, cht); err != nil {
			log.Error(usr, err, "failed updating chat_id")
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		log.Error(usr, err, "failed committing Tx for adding user")
		return err
	}

	return nil
}

func AddMovie(usr int64, title string) error {
	query := `INSERT INTO movies (title, created_on, created_by) VALUES ($1, $2, $3)`
	if _, err := db.Exec(query, title, clk.Now().UTC(), usr); err != nil {
		log.Error(usr, err, "failed inserting movie")
		return err
	}

	return nil
}

func DelMovie(usr int64, movieID int) error {
	if _, err := db.Exec(`DELETE FROM movies WHERE id=$1`, movieID); err != nil {
		log.Error(usr, err, "failed deleting movie")
		return err
	}

	return nil
}

func RandomMovie(usr int64) (string, error) {
	log.Warn(usr, "RandomMovie is not implemented")
	return "Not implemented", nil
}

func RateMovie(usr int64, movieID int, rating int) error {
	var r int
	err := db.QueryRow(`SELECT rating FROM ratings WHERE user_id=$1 AND movie_id=$2`, usr, movieID).Scan(&r)
	switch {
	case err == sql.ErrNoRows:
		query := `INSERT INTO ratings (user_id, movie_id, rating, created_on) VALUES ($1, $2, $3, $4)`
		if _, err := db.Exec(query, usr, movieID, rating, clk.Now().UTC()); err != nil {
			log.Errorf(usr, err, "failed rating movie %d", movieID)
			return err
		}

	case err != nil:
		log.Errorf(usr, err, "failed rating movie %d", movieID)
		return err

	default:
		if r == rating {
			return nil
		}

		query := `UPDATE ratings SET rating=$1, updated_on=$2 WHERE user_id=$3 AND movie_id=$4`
		if _, err := db.Exec(query, rating, clk.Now().UTC(), usr, movieID); err != nil {
			log.Errorf(usr, err, "failed rating movie %d", movieID)
			return err
		}
	}

	return nil
}

func UnrateMovie(usr int64, movie int) (bool, error) {
	var r int
	err := db.QueryRow(`SELECT rating FROM ratings WHERE user_id=$1 AND movie_id=$2`, usr, movie).Scan(&r)
	switch {
	case err == sql.ErrNoRows:
		return false, nil

	case err != nil:
		log.Errorf(usr, err, "failed unrating movie %d", movie)
		return false, err

	default:
		query := `DELETE FROM ratings WHERE user_id=$1 AND movie_id=$2`
		if _, err := db.Exec(query, usr, movie); err != nil {
			log.Errorf(usr, err, "failed unrating movie %d", movie)
			return false, err
		}
	}

	return true, nil
}

type MovieState int

type Movie struct {
	ID     int
	Title  string
	Year   int16
	Rating float32
}

const (
	MovieStateSeen MovieState = iota
	MovieStateUnseen
	MovieStateAll
)

func listMovies(usr int64, rows *sql.Rows) ([]Movie, error) {
	var err error = nil
	movies := []Movie{}
	for rows.Next() {
		var m Movie
		var year sql.NullInt16
		var rating sql.NullFloat64
		if err = rows.Scan(&m.ID, &m.Title, &year, &rating); err != nil {
			log.Error(usr, err, "couldn't read attributes of a movie")
			continue
		}

		if year.Valid {
			m.Year = year.Int16
		} else {
			m.Year = -1
		}

		if rating.Valid {
			m.Rating = float32(rating.Float64)
		} else {
			m.Rating = -1
		}

		movies = append(movies, m)
	}

	return movies, err
}

func ListSeenMovies(usr int64) ([]Movie, error) {
	query := `SELECT m.id AS ID, m.title AS title, m.year AS year, r1.avg_rating AS avg_rating
FROM movies m JOIN ratings r2 ON m.id=r2.movie_id AND r2.user_id=$1 JOIN (
	SELECT movie_id, AVG(rating) AS avg_rating
	FROM ratings
	GROUP BY movie_id
) r1 ON m.id=r1.movie_id
ORDER BY avg_rating DESC;`
	rows, err := db.Query(query, usr)
	if err != nil {
		log.Error(usr, err, "failed querying seen movies")
		return []Movie{}, nil
	}
	defer rows.Close()

	return listMovies(usr, rows)
}

func ListUnseenMovies(usr int64) ([]Movie, error) {
	query := `SELECT m.id AS ID, m.title AS title, m.year AS year, r1.avg_rating AS avg_rating
FROM movies m
	LEFT JOIN (
		SELECT movie_id, AVG(rating) AS avg_rating
		FROM ratings
		GROUP BY movie_id
	) r1 ON m.id=r1.movie_id
	LEFT JOIN (
		SELECT movie_id
		FROM ratings
		WHERE user_id=$1
	) r2 ON m.id=r2.movie_id
WHERE r2.movie_id IS NULL
ORDER BY avg_rating DESC`
	rows, err := db.Query(query, usr)
	if err != nil {
		log.Error(usr, err, "failed querying seen movies")
		return []Movie{}, nil
	}
	defer rows.Close()

	return listMovies(usr, rows)
}

func ListAllMovies(usr int64) ([]Movie, error) {
	query := `SELECT m.id AS ID, m.title AS title, m.year AS year, r.avg_rating AS avg_rating
FROM movies m LEFT JOIN (
	SELECT movie_id, AVG(rating) AS avg_rating
	FROM ratings
	GROUP BY movie_id
) r ON m.id=r.movie_id
ORDER BY avg_rating DESC`
	rows, err := db.Query(query)
	if err != nil {
		log.Error(usr, err, "failed querying seen movies")
		return []Movie{}, nil
	}
	defer rows.Close()

	return listMovies(usr, rows)
}

func ListTopMovies(usr int64) ([]Movie, error) {
	panic("Not implemented")
}

func ListLatestMovies(usr int64) ([]Movie, error) {
	panic("Not implemented")
}
