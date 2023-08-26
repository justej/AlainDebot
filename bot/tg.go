package bot

import (
	"AlainDebot/db"
	"AlainDebot/log"
	"fmt"
	"strconv"
	"strings"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var bot *tg.BotAPI

func Init(token string) {
	b, err := tg.NewBotAPI(token)
	if err != nil {
		panic(err)
	}

	b.Debug = false
	log.Infof("Successfully initialized bot as %s", b.Self.UserName)

	bot = b
}

func Run() {
	uCfg := tg.NewUpdate(0)
	uCfg.Timeout = 60

	for u := range bot.GetUpdatesChan(uCfg) {
		if u.Message != nil {
			if u.Message.IsCommand() {
				go handleCommand(&u)
			} else {
				go handleUpdate(&u)
			}
		}
	}

}

type command struct {
	name string
	len  int
}

var (
	cmdStart   = makeCommand("start")
	cmdAdd     = makeCommand("add")
	cmdDel     = makeCommand("del")
	cmdRate    = makeCommand("rate")
	cmdUnrate  = makeCommand("unrate")
	cmdAmazeMe = makeCommand("amazeme")
	cmdUnseen  = makeCommand("unseen")
	cmdSeen    = makeCommand("seen")
	cmdAll     = makeCommand("all")
	cmdTop     = makeCommand("top")
	cmdLatest  = makeCommand("latest")
	cmdFind    = makeCommand("find")
	cmdHelp    = makeCommand("help")
)

type state uint

var states = make(map[int64]state)

const (
	stateIdle state = iota
	stateAdd
	stateDel
	stateRate
	stateUnrate
	stateFind
)

func makeCommand(c string) *command {
	return &command{
		name: c,
		len:  len(c) + 2,
	}
}

func handleCommand(upd *tg.Update) {
	msg := upd.Message
	cmd := msg.Command()
	usr := msg.From.ID
	cht := msg.Chat.ID

	switch cmd {
	case cmdStart.name:
		err := db.AddUser(usr, cht)
		if err != nil {
			log.Error(usr, err, "failed adding user")
			return
		}

		m := tg.NewMessage(cht, "Welcome! Now you can add movies you would like to watch and rate your and other's movies. To see the list of available commands send /help. Happy watching!")
		if _, err := bot.Send(m); err != nil {
			log.Error(usr, err, "failed sending response to user")
			return
		}

	case cmdAdd.name:
		m := tg.NewMessage(cht, "What's the title of the movie?")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
		}

		states[usr] = stateAdd

	case cmdDel.name:
		txt, ok := listAllMovies(usr, cmd, "Pick the victim\n\n")
		if !ok {
			return
		}

		m := tg.NewMessage(cht, txt)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		states[usr] = stateDel

	case cmdRate.name:
		txt, ok := listAllMovies(usr, cmd, "Send me comma separated ID of the movie and the rate (1 to 5), e.g: 42, 5 or 42, 3\n\n")
		if !ok {
			return
		}

		m := tg.NewMessage(cht, txt)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		states[usr] = stateRate

	case cmdUnrate.name:
		lst, err := db.ListSeenMovies(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		var rated string
		if len(lst) == 0 {
			rated = "There's nothing to unrate"
		} else {
			rated = joinMovies(lst, "Pick the ID of the movie to unrate:\n\n")
		}

		m := tg.NewMessage(cht, rated)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		states[usr] = stateUnrate

	case cmdAmazeMe.name:
		randomMovie, err := db.RandomMovie(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		m := tg.NewMessage(cht, randomMovie)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdUnseen.name:
		lst, err := db.ListUnseenMovies(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		var unseen string
		if len(lst) == 0 {
			unseen = "Wow, you've already seen all movies on the list!"
		} else {
			unseen = joinMovies(lst, "You haven't seen these yet:\n\n")
		}

		m := tg.NewMessage(cht, unseen)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdSeen.name:
		listNotFull := ""
		lst, err := db.ListSeenMovies(usr)
		if err != nil {
			if len(lst) == 0 {
				log.Errorf(usr, err, "failed handling '%s'", cmd)
				return
			}

			listNotFull = "Just in case, this list may be incomplete (sorry for technical issues).\n"
		}

		var seen string
		if len(lst) == 0 {
			seen = "Hm, you have seen none from the list"
		} else {
			seen = joinMovies(lst, listNotFull, "You have seen only these movies:\n\n")
		}
		m := tg.NewMessage(cht, seen)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdAll.name:
		lst, err := db.ListAllMovies(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		m := tg.NewMessage(cht, joinMovies(lst, "All movies\n\n"))
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdTop.name:
		lst, err := db.ListTopMovies(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		m := tg.NewMessage(cht, joinMovies(lst, "Top 10 movies\n\n"))
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdLatest.name:
		lst, err := db.ListLatestMovies(usr)
		if err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

		m := tg.NewMessage(cht, joinMovies(lst, "10 latest movies\n\n"))
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdFind.name:
		m := tg.NewMessage(cht, "Not implemented yet")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}

	case cmdHelp.name:
		help := `Available commands:
/add - add new movie
/del - delete movie
/rate - rate movie
/unrate - unrate movie
/amazeme - show a random unseen movie
/unseen - list unseen movies
/seen - list seen movies
/all - list both unseen and seen movies
/top - list top 10 movies
/latest - list 10 latest movies
/find - find movie by name or year
/help - this help`
		m := tg.NewMessage(cht, help)
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling '%s'", cmd)
			return
		}
	}
}

func listAllMovies(usr int64, cmd, suffix string) (string, bool) {
	movies, err := db.ListAllMovies(usr)
	if err != nil {
		log.Errorf(usr, err, "failed handling '%s'", cmd)
		return "", false
	}

	return joinMovies(movies, suffix), true
}

func joinMovies(movies []db.Movie, suffix ...string) string {
	var sb strings.Builder
	sb.Grow(len(movies) * 20)
	sb.WriteString(strings.Join(suffix, ""))
	for _, movie := range movies {
		var line string
		if movie.Rating < 0 {
			line = fmt.Sprintf("%2d: %s (no ⭐ yet)\n", movie.ID, movie.Title)
		} else {
			line = fmt.Sprintf("%2d: %s (%.2f ⭐)\n", movie.ID, movie.Title, movie.Rating)
		}
		sb.WriteString(line)
	}

	return sb.String()
}

func handleUpdate(upd *tg.Update) {
	msg := upd.Message
	usr := msg.From.ID
	cht := msg.Chat.ID
	state := states[usr]

	switch state {
	case stateIdle:
		m := tg.NewMessage(cht, "What does this supposed to mean? Pick a command maybe?")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
			return
		}

	case stateAdd:
		title := strings.TrimSpace(msg.Text)
		err := db.AddMovie(usr, title)
		if err != nil {
			log.Errorf(usr, err, "failed adding movie '%s'", title)
			return
		}

		states[usr] = stateIdle

		m := tg.NewMessage(cht, "Done")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
			return
		}

	case stateDel:
		id, err := strconv.Atoi(strings.TrimSpace(msg.Text))
		if err != nil {
			log.Warn(usr, "movie ID is not an integer")

			m := tg.NewMessage(cht, "There's no movie with such ID. Try better")
			if _, err := bot.Send(m); err != nil {
				log.Error(usr, err, "failed sending error back")
				states[usr] = stateIdle
			}
			return
		}

		err = db.DelMovie(usr, id)
		if err != nil {
			log.Errorf(usr, err, "failed deleting movie '%d'", id)
			return
		}

		states[usr] = stateIdle

		m := tg.NewMessage(cht, "Done")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
			return
		}

	case stateRate:
		parts := strings.Split(msg.Text, ",")
		if len(parts) != 2 {
			m := tg.NewMessage(cht, "I only deal with two comma separated values. Try again")
			if _, err := bot.Send(m); err != nil {
				log.Error(usr, err, "failed sending rate error back")
			}
			states[usr] = stateIdle
			return
		}

		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			log.Warn(usr, "movie ID is not an integer")

			m := tg.NewMessage(cht, "There's no movie with such ID. Try better")
			if _, err := bot.Send(m); err != nil {
				log.Error(usr, err, "failed sending rate error back")
			}
			states[usr] = stateIdle
			return
		}

		rate, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || rate < 1 || rate > 5 {
			log.Warn(usr, "rate value is not a number or out of range")

			m := tg.NewMessage(cht, "Rate has to be an integer number from 1 to 5")
			if _, err := bot.Send(m); err != nil {
				log.Error(usr, err, "failed sending error back")
			}
			states[usr] = stateIdle
			return
		}

		err = db.RateMovie(usr, id, rate)
		if err != nil {
			log.Errorf(usr, err, "failed rating movie '%d'", id)
			return
		}

		states[usr] = stateIdle

		m := tg.NewMessage(cht, "Done")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
			return
		}

	case stateUnrate:
		id, err := strconv.Atoi(strings.TrimSpace(msg.Text))
		if err != nil {
			log.Warn(usr, "movie ID is not an integer")

			m := tg.NewMessage(cht, "There's no movie with such ID. Try better")
			if _, err := bot.Send(m); err != nil {
				log.Error(usr, err, "failed sending rate error back")
			}
			states[usr] = stateIdle
			return
		}

		done, err := db.UnrateMovie(usr, id)
		if err != nil {
			log.Errorf(usr, err, "failed unrating movie '%d'", id)
			return
		}

		states[usr] = stateIdle

		if !done {
			m := tg.NewMessage(cht, "It seems you've entered a wrong movie ID. Check it out")
			if _, err := bot.Send(m); err != nil {
				log.Errorf(usr, err, "failed handling state '%d'", state)
			}
			return
		}

		m := tg.NewMessage(cht, "Done")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
			return
		}

	case stateFind:
		m := tg.NewMessage(cht, "Not implemented")
		if _, err := bot.Send(m); err != nil {
			log.Errorf(usr, err, "failed handling state '%d'", state)
		}

		states[usr] = stateIdle
	}
}
