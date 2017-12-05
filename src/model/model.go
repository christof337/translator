package model

import (
	"golang.org/x/crypto/bcrypt"
	"fmt"
	"math/rand"
	// "sort"
	// "strconv"
	"strings"
	// "time"
)

// Languages
var Languages []string = []string{
	"gb", "it", "fr", "de", "es", "pl", "pt", "ru", "nl", "no", "jp", "cy", "kl", "us",
}

var DisplayLanguages []string = []string{
	"it", "de", "es", "fr", "pl", "pt", "ru", "nl", "jp",
}

var LanguageNames map[string]string = map[string]string{
	"gb": "English",
	"it": "Italiano",
	"fr": "Français",
	"de": "Deutsch",
	"es": "Español",
	"pl": "Polski",
	"pt": "Português",
	"br": "Português do Brasil",
	"ru": "Ру́сский",
	"nl": "Nederlands",
	"no": "Norsk",
	"jp": "日本語",
	"cy": "Cymraeg",
	"kl": "Klingon",
	"us": "US English",
}

var LanguageNamesEnglish map[string]string = map[string]string{
	"gb": "English",
	"it": "Italian",
	"fr": "French",
	"de": "German",
	"es": "Spanish",
	"pl": "Polish",
	"pt": "Portuguese",
	"br": "Brazilian Portuguese",
	"ru": "Russian",
	"nl": "Dutch",
	"no": "Norwegian",
	"jp": "Japanese",
	"cy": "Welsh",
	"kl": "Klingon",
	"us": "US English",
}

var LanguagePaths map[string]string = map[string]string{
	"it": "italian",
	"fr": "french",
	"de": "german",
	"es": "spanish",
	"pl": "polish",
	"pt": "portuguese",
	"br": "brazilian",
	"ru": "russian",
	"nl": "dutch",
	"no": "norwegian",
	"jp": "japanese",
	"cy": "welsh",
	"kl": "klingon",
	"us": "american",
}

var LevelNames []string = []string{
	"All", "Core", "Advanced", "Further", "Third Party",
}

//  completion

func GetLanguageCompletion() map[string][4]int {
	var completion = make(map[string][4]int, len(Languages))
	var totals [4]int
	for i := 1; i <= 4; i++ {
		totals[i-1] = query("select sum(EntrySources.Count) from EntrySources "+
			"inner join Entries on Entries.EntryID = EntrySources.EntryID "+
			"inner join Sources on Sources.SourceID = EntrySources.SourceID "+
			"where Sources.Level = ?", i).count()
	}

	for _, lang := range Languages {
		if lang == "gb" {
			completion[lang] = [4]int{100, 100, 100, 100}
		} else {
			var values [4]int
			for i := 1; i <= 4; i++ {
				count := query("select sum(EntrySources.Count) from EntrySources "+
					"inner join Entries on Entries.EntryID = EntrySources.EntryID "+
					"inner join Sources on Sources.SourceID = EntrySources.SourceID "+
					"where Sources.Level = ? and Entries.EntryID in "+
					"(select EntryID from Translations where Language = ?)", i, lang).count()
				if totals[i-1] > 0 {
					values[i-1] = 100 * count / totals[i-1]
				}
				fmt.Println("Completion of", LanguageNames[lang], "@", i, "=", count, "/", totals[i-1])
			}
			completion[lang] = values
		}
	}
	return completion
}

func WhereNotMe(in chan string) <-chan string {
	out := make(chan string)
	for s := range in {
		if s != "me" {
			out <- s
		}
	}
	return out
}

// sort entries by index
type entriesByIndex []*Entry

func (this entriesByIndex) Len() int {
	return len(this)
}
func (this entriesByIndex) Less(i, j int) bool {
	return strings.Index(this[i].PartOf, this[i].Original) < strings.Index(this[j].PartOf, this[j].Original)
}
func (this entriesByIndex) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

//

func (user *User) GenerateSecret() string {
	base := make([]byte, 256)
	for i, _ := range base {
		base[i] = byte(rand.Int())
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(base), bcrypt.MinCost)
	if err != nil {
		fmt.Println("Error generating secret hash:", err)
		return ""
	}
	secret := string(bytes)
	// secret = secret[7:]
	fmt.Println("Generate secret:", secret)

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
	if err != nil {
		fmt.Println("Error generating secret hash:", err)
	}
	fmt.Println("Generate secret: hash:", string(hash))
	user.Secret = string(hash)
	user.Save()

	return secret
}

func (user *User) SetLanguageLead() {
	query("update Users set IsLanguageLead = 0 where Language = ?", user.Language).exec()
	query("update Users set IsLanguageLead = 1 where Email = ?", user.Email).exec()
}

func (user *User) ClearLanguageLead() {
	query("update Users set IsLanguageLead = 0 where Email = ?", user.Email).exec()
}
