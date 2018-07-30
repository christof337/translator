package main

import (
	"./config"
	"./model"
	// "database/sql"
	"fmt"
	_ "github.com/ziutek/mymysql/godrv"
)

func main() {
	dbname1 := config.Config.OldDatabase.Database
	dbname2 := config.Config.Database.Database
	// dbname1 := "chartrans"
	// dbname2 := "chartrans2"
	// dbuser := "chartrans"
	// dbpassword := "fiddlesticks"

	// db1, err := sql.Open("mymysql", dbname1+"/"+dbuser+"/"+dbpassword)
	db1, err := config.Config.OldDatabase.Open()
	if err != nil {
		fmt.Println("Error opening database 1:", err)
	}
	// db2, err := sql.Open("mymysql", dbname2+"/"+dbuser+"/"+dbpassword)
	db2, err := config.Config.Database.Open()
	if err != nil {
		fmt.Println("Error opening database 2:", err)
	}

	// clear out
	if !config.Config.Partial {
		fmt.Print("Clearing out old data... ")
		_, _ = db2.Exec("delete from Entries")
		_, _ = db2.Exec("delete from Sources")
		_, _ = db2.Exec("delete from EntrySources")
		_, _ = db2.Exec("delete from Translations")
		_, _ = db2.Exec("delete from Users")
		_, _ = db2.Exec("delete from Votes")
		fmt.Println("done.")
	}

	// entries
	fmt.Print("Entries... ")
	rows, err := db1.Query("select Original, PartOf from Entries")
	if err != nil {
		fmt.Println("Error reading entries:", err)
	} else {
		n_entries := 0
		for rows.Next() {
			entry := model.Entry{}
			err = rows.Scan(&entry.Original, &entry.PartOf)
			if err != nil {
				fmt.Println("Error reading entry:", err)
				continue
			}

			if config.Config.Partial && model.RecordExists("Entries", map[string]interface{}{
				"EntryID": entry.ID(),
			}) {
				continue
			}

			_, err = db2.Exec("insert into Entries(EntryID, Original, PartOf) values (?,?,?)",
				entry.ID(), entry.Original, entry.PartOf)
			if err != nil {
				fmt.Println("Error writing entry:", err)
				continue
			}
			n_entries++
		}
		fmt.Println(n_entries)
	}
	rows.Close()

	// sources
	fmt.Print("Sources... ")
	rows, err = db1.Query("select Filepath, Page, Volume, Level, Game from Sources")
	if err != nil {
		fmt.Println("Error reading sources:", err)
	} else {
		n_sources := 0
		for rows.Next() {
			source := model.Source{}
			err = rows.Scan(&source.Filepath, &source.Page, &source.Volume, &source.Level, &source.Game)
			if err != nil {
				fmt.Println("Error reading srouce:", err)
				continue
			}

			if config.Config.Partial && model.RecordExists("Sources", map[string]interface{}{
				"SourceID": source.ID(),
			}) {
				continue
			}

			_, err = db2.Exec("insert into Sources(SourceID, Filepath, Page, Volume, Level, Game) values (?, ?, ?, ?, ?, ?)",
				source.ID(), source.Filepath, source.Page, source.Volume, source.Level, source.Game)
			if err != nil {
				fmt.Println("Error writing source:", err)
				continue
			}
			n_sources++
		}
		fmt.Println(n_sources)
	}

	// result, err := db2.Exec("insert into Sources (SourceID, Filepath, Page, Volume, Level, Game) select Filepath, Page, Volume, Level, Game from " + dbname1 + ".Sources")
	// if err != nil {
	// 	fmt.Println("Error transferring sources:", err)
	// } else {
	// 	n_sources, _ := result.RowsAffected()
	// 	fmt.Println(n_sources)
	// }

	fmt.Print("Source lines... ")
	rows, err = db1.Query("select EntryOriginal, EntryPartOf, SourcePath, Count from EntrySources")
	if err != nil {
		fmt.Println("Error reading source lines:", err)
	} else {
		n_es := 0
		for rows.Next() {
			es := model.EntrySource{}
			err = rows.Scan(&es.Entry.Original, &es.Entry.PartOf, &es.Source.Filepath, &es.Count)
			if err != nil {
				fmt.Println("Error reading source line:", err)
			}

			if config.Config.Partial && model.RecordExists("EntrySources", map[string]interface{}{
				"EntryID": es.Entry.ID(),
				"SourceID": es.Source.ID(),
			}) {
				continue
			}

			_, err = db2.Exec("insert into EntrySources(EntryID, SourceID, Count) values (?,?,?)", es.Entry.ID(), es.Source.ID(), es.Count)
			if err != nil {
				fmt.Println("Error writing source line:", err)
				continue
			}
			n_es++
		}
		fmt.Println(n_es)
	}
	rows.Close()

	// translations
	fmt.Print("Translations... ")
	rows, err = db1.Query("select EntryOriginal, EntryPartOf, Language, Translator, Translation, IsPreferred from Translations")
	if err != nil {
		fmt.Println("Error reading translations:", err)
	} else {
		n_translations := 0
		for rows.Next() {
			translation := model.Translation{}
			err = rows.Scan(&translation.Entry.Original, &translation.Entry.PartOf, &translation.Language, &translation.Translator, &translation.Translation, &translation.IsPreferred)
			if err != nil {
				fmt.Println("Error reading translation:", err)
				continue
			}

			if translation.Entry.Original == "(Round down)" && translation.Language == "pl" {
				fmt.Println(" *** Transferring: (Round down) =", translation.Translation)
			}

			if config.Config.Partial && model.RecordExists("Translations", map[string]interface{}{
				"TranslationID": translation.ID(),
			}) {
				continue
			}

			_, err = db2.Exec("insert into Translations(TranslationID, EntryID, Language, Translator, Translation, IsPreferred, IsConflicted) values (?,?,?,?,?,?,?)",
				translation.ID(), translation.Entry.ID(), translation.Language, translation.Translator, translation.Translation, translation.IsPreferred, false)
			n_translations++
		}
		fmt.Println(n_translations)
	}
	rows.Close()

	// users
	fmt.Print("Users... ")
	result, err := db2.Exec("insert into Users (Email, Password, Secret, Name, IsAdmin, Language, IsLanguageLead) select Email, Password, Secret, Name, IsAdmin, Language, IsLanguageLead from " + dbname1 + ".Users where Email not in (select Email from " + dbname2 + ".Users)")
	if err != nil {
		fmt.Println("Error transferring users:", err)
	} else {
		n_users, _ := result.RowsAffected()
		fmt.Println(n_users)
	}

	// votes
	fmt.Print("Votes...")
	rows, err = db1.Query("select EntryOriginal, EntryPartOf, Language, Translator, Voter, Vote from Votes")
	if err != nil {
		fmt.Println("Error reading votes:", err)
	} else {
		n_votes := 0
		for rows.Next() {
			vote := model.Vote{}
			var voter string
			err = rows.Scan(&vote.Translation.Entry.Original, &vote.Translation.Entry.PartOf, &vote.Translation.Language, &vote.Translation.Translator, &voter, &vote.Vote)
			if err != nil {
				fmt.Println("Error reading vote:", err)
				continue
			}


			if config.Config.Partial && model.RecordExists("Votes", map[string]interface{}{
				"TranslationID": vote.Translation.ID(),
				"Voter": voter,
			}) {
				continue
			}

			_, err = db2.Exec("insert into Votes (TranslationID, Voter, Vote) values (?,?,?)", vote.Translation.ID(), voter, vote.Vote)
			if err != nil {
				fmt.Println("Error writing vote:", err)
				continue
			}
			n_votes++
		}
		fmt.Println(n_votes)
	}

	// conflicts
	fmt.Print("Finding conflicts...")
	model.MarkAllConflicts()
}
