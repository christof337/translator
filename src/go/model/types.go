package model

import (
	"crypto/md5"
	"database/sql"
	// "encoding/hex"
	// "encoding/binary"
	"fmt"
	"github.com/ziutek/mymysql/mysql"
	"strings"
)

func hash64(data string) uint64 {
	hasher := md5.New()
	hasher.Write([]byte(data))
	hash := hasher.Sum(nil)

	hash64 :=
		uint64(hash[0])<<24 +
			uint64(hash[1])<<16 +
			uint64(hash[2])<<8 +
			uint64(hash[3])
	return hash64
}

func parseID(rows *sql.Rows) (Result, error) {
	var id uint64
	err := rows.Scan(&id)
	return id, err
}

// ** Entries

type Entry struct {
	Original string
	PartOf   string
}

func (entry *Entry) ID() uint64 {
	if entry == nil {
		return 0
	}

	var str = entry.Original
	if entry.PartOf != "" && entry.PartOf != entry.Original {
		str = entry.Original + "  ----  " + entry.PartOf
	}
	return hash64(str)
}

func DeleteAllEntrySources() {
	if Debug >= 2 {
		fmt.Println(" ***** Deleting ALL entry sources")
	}
	// if ok := query("delete from Entries").exec(); !ok {
	// 	fmt.Println(" ***** Error deleting entries")
	// }
	if ok := query("delete from Sources").exec(); !ok {
		fmt.Println(" ***** Error deleting sources")
	}
	if ok := query("delete from EntrySources").exec(); !ok {
		fmt.Println(" ***** Error deleting entrysources")
	}
	if Debug >= 2 {
		fmt.Println(" ***** Deleted ALL entry sources")
	}
}

const entryFields = "Original, PartOf"

func parseEntry(rows *sql.Rows) (Result, error) {
	e := Entry{}
	err := rows.Scan(&e.Original, &e.PartOf)
	// fmt.Println("Entry ID: " + e.ID() + " (" + string(len(e.ID())) + ")")
	return e, err
}

func makeEntries(results []Result) []*Entry {
	entries := make([]*Entry, len(results))
	for i, result := range results {
		if entry, ok := result.(Entry); ok {
			entries[i] = &entry
		}
	}
	return entries
}

func CountEntries() int {
	return query("select count(*) from Entries").count()
}

func GetEntryByID(id string) *Entry {
	result := query("select "+entryFields+" from Entries where EntryID = ?", id).row(parseEntry)
	if entry, ok := result.(Entry); ok {
		return &entry
	}
	return nil
}

func GetEntries() []*Entry {
	results := query("select " + entryFields + " from Entries").rows(parseEntry)
	return makeEntries(results)
}

func GetEntriesPartOf(partOf string) []*Entry {
	results := query("select " + entryFields + " from Entries where Partof = ?", partOf).rows(parseEntry)
	return makeEntries(results)
}

func GetEntriesAt(game string, level int, show, search string, fuzzySearch bool, language string, translator *User) []*Entry {
	if game == "" && level == 0 && show == "" && search == "" {
		return GetEntries()
	}
	args := make([]interface{}, 0, 2)
	sql := "select Original, PartOf from Entries " +
		"inner join EntrySources on Entries.EntryID = EntrySources.EntryID " +
		"inner join Sources on EntrySources.SourceID = Sources.SourceID"
	if show == "conflicts" {
		sql = sql + " inner join Translations on Entries.EntryID = Translations.EntryID and Translations.Language = ?"
		args = append(args, language)
		// sql = sql + " inner join Translations Mine on EntryID = Mine.EntryID and Mine.Language = ? and Mine.Translator = ?" +
		// 	"inner join Translations Others on Entries.EntryID = Others.EntryID and Others.Language = ? and Others.Translator != ?"
		// args = append(args, language)
		// args = append(args, translator.Email)
		// args = append(args, language)
		// args = append(args, translator.Email)
	} else if show == "mine" {
		sql = sql + " inner join Translations Mine on Entries.EntryID = Mine.EntryID and Mine.Language = ? and Mine.Translator = ?"
		args = append(args, language)
		args = append(args, translator.Email)
	} else if show == "others" {
		sql = sql + " inner join Translations Others on Entries.EntryID = Others.EntryID and Others.Language = ? and Others.Translator = ?"
		args = append(args, language)
		args = append(args, translator.Email)
	} else if show != "" {
		sql = sql + " left join Translations on Entries.EntryID = Translations.EntryID and Translations.Language = ?"
		args = append(args, language)
	}
	sql = sql + " where 1 = 1"

	if game != "" {
		if game == "dnd35" {
			game = "3.5"
		}
		sql = sql + " and Game = ?"
		args = append(args, game)
	}
	if level != 0 {
		sql = sql + " and Level = ?"
		args = append(args, level)
	}
	if show == "conflicts" {
		sql = sql + " and Translations.IsConflicted = 1"
	}
	// if show != "" {
	// 	sql = sql+" and Translations.Language = ?"
	// 	args = append(args, language)
	// }
	if search != "" {
		searchTerms := strings.Split(search, " ")
		fmt.Println("Searching for:", search)

		if fuzzySearch {
			// todo make it more fuzzy?
			sql = sql + " and ("
			first := true
			for _, term := range searchTerms {
				if first {
					first = false
				} else {
					sql = sql + " or "
				}
				term = strings.ToLower(term)
				sql = sql + "lower(Original) like ?"
				args = append(args, "%"+term+"%")
			}
			sql = sql + ")"
		} else {
			for _, term := range searchTerms {
				term = strings.ToLower(term)
				sql = sql + " and lower(Original) like ?"
				args = append(args, "%"+term+"%")
			}
		}
	}

	sql = sql + " group by Entries.EntryID"
	if show == "translated" {
		sql = sql + " having count(Translations.Translation) > 0"
	} else if show == "untranslated" {
		sql = sql + " having count(Translations.Translation) = 0"
	}
	fmt.Println("Get entries:", sql)
	results := query(sql, args...).rows(parseEntry)
	return makeEntries(results)
}

func (entry *Entry) Save() {
	keyfields := map[string]interface{}{
		"EntryID": entry.ID(),
	}
	fields := map[string]interface{}{
		"Original": entry.Original,
		"PartOf":   entry.PartOf,
	}
	saveRecord("Entries", keyfields, fields)
}

func (entry *Entry) CountTranslations() map[string]int {
	counts := make(map[string]int, len(Languages))
	query("select Language, Count(*) from Translations where EntryID = ? group by Language", entry.ID()).rows(func(rows *sql.Rows) (Result, error) {
		var language string
		var count int
		rows.Scan(&language, &count)
		counts[language] = count
		return nil, nil
	})
	return counts
}

func (entry *Entry) GetParts() []*Entry {
	if entry.PartOf == "" {
		entries := make([]*Entry, 1)
		entries[0] = entry
		return entries
	}
	results := query("select "+entryFields+" from Entries where PartOf = ?", entry.PartOf).rows(parseEntry)
	return makeEntries(results)
}

// ** Sources

type Source struct {
	Filepath string
	Page     string
	Volume   string
	Level    int
	Game     string
}

func GetSourceByID(id string) *Source {
	result := query("select "+sourceFields+" from Sources where SourceID = ?", id).row(parseSource)
	if source, ok := result.(Source); ok {
		return &source
	}
	return nil
}

func (source *Source) ID() uint64 {
	if source == nil {
		return 0
	}

	return hash64(source.Filepath)
	// hasher := md5.New()
	// hasher.Write([]byte(source.Filepath))
	// return hex.EncodeToString(hasher.Sum(nil))
}

func parseSource(rows *sql.Rows) (Result, error) {
	s := Source{}
	err := rows.Scan(&s.Filepath, &s.Page, &s.Volume, &s.Level, &s.Game)
	return s, err
}

const sourceFields = "Filepath, Page, Volume, Level, Game"

func GetSources() []*Source {
	results := query("select " + sourceFields + " from Sources").rows(parseSource)

	sources := make([]*Source, len(results))
	for i, result := range results {
		if source, ok := result.(Source); ok {
			sources[i] = &source
		}
	}
	return sources
}
func GetSourcesAt(game string, level int, show string) []*Source {
	if game == "" && level == 0 && show == "" {
		return GetSources()
	}
	args := make([]interface{}, 0, 2)
	sql := "select " + sourceFields + " from Sources "

	sql = sql + " where 1 = 1"

	if game != "" {
		if game == "dnd35" {
			game = "3.5"
		}
		sql = sql + " and Game = ?"
		args = append(args, game)
	}
	if level != 0 {
		sql = sql + " and Level = ?"
		args = append(args, level)
	}

	// sql = sql+" group by Original, PartOf"
	if show == "translated" || show == "untranslated" {
		sql = sql + " and Sources.SourceID"
		if show == "untranslated" {
			sql = sql + " not"
		}
		sql = sql + " in (select EntrySources.SourceID from EntrySources" +
			" inner join Translations on EntrySources.EntryID = Translations.EntryID)"
	}

	fmt.Println("Get entries:", sql)
	results := query(sql, args...).rows(parseSource)

	sources := make([]*Source, 0, len(results))
	for _, result := range results {
		if source, ok := result.(Source); ok {
			sources = append(sources, &source)
		}
	}
	return sources
}

func (source *Source) Save() {
	keyfields := map[string]interface{}{
		"SourceID": source.ID(),
	}
	fields := map[string]interface{}{
		"Filepath": source.Filepath,
		"Page":   source.Page,
		"Volume": source.Volume,
		"Level":  source.Level,
		"Game":   source.Game,
	}
	saveRecord("Sources", keyfields, fields)
}

func (source *Source) GetLanguageCompletion() map[string]int {
	var completion = make(map[string]int, len(Languages))

	total := query("select count(distinct Entries.EntryID) from Entries "+
		"inner join EntrySources on Entries.EntryID = EntrySources.EntryID "+
		"where EntrySources.SourceID = ?", source.ID()).count()
	if total > 0 {
		for _, lang := range Languages {
			count := query("select count(distinct Translations.EntryID) from Translations "+
				"inner join EntrySources on Translations.EntryID = EntrySources.EntryID "+
				"where EntrySources.SourceID = ? and Language = ?", source.ID(), lang).count()
			completion[lang] = 100 * count / total
		}
	}
	return completion
}

type EntrySource struct {
	Entry  Entry
	Source Source
	Count  int
}

func parseEntrySource(rows *sql.Rows) (Result, error) {
	es := EntrySource{}
	var entryID string
	var sourceID string
	err := rows.Scan(&entryID, &sourceID, &es.Count)
	if entry := GetEntryByID(entryID); entry == nil {
		return nil, nil
	} else {
		es.Entry = *entry
	}
	if source := GetSourceByID(sourceID); source == nil {
		return nil, nil
	} else {
		es.Source = *source
	}
	return es, err
}

const entrySourceFields = "EntryID, SourceID, Count"

func GetEntrySources() []*EntrySource {
	results := query("select EntryID, EntrySources.SourceID, Count" +
		" from EntrySources inner join Sources on EntrySources.SourceID = Sources.SourceID").rows(parseEntrySource)

	sources := make([]*EntrySource, len(results))
	for i, result := range results {
		if source, ok := result.(EntrySource); ok {
			sources[i] = &source
		}
	}
	return sources
}

func GetSourcesForEntry(entry *Entry) []*EntrySource {
	results := query("select EntryID, EntrySources.SourceID, Count"+
		" from EntrySources inner join Sources on EntrySources.SourceID = Sources.SourceID "+
		"where EntryID = ?", entry.ID()).rows(parseEntrySource)

	sources := make([]*EntrySource, len(results))
	for i, result := range results {
		if source, ok := result.(EntrySource); ok {
			sources[i] = &source
		}
	}
	return sources
}

func (es *EntrySource) Save() {
	keyfields := map[string]interface{}{
		"EntryID":    es.Entry.ID(),
		"SourceID": es.Source.ID(),
	}
	fields := map[string]interface{}{
		"Count": es.Count,
	}
	saveRecord("EntrySources", keyfields, fields)
}

type EntrySourcePlaceholder struct {
	SourceID uint64
	Count    int
}

func parseEntrySourcePlaceholder(rows *sql.Rows) (Result, error) {
	placeholder := EntrySourcePlaceholder{}
	err := rows.Scan(&placeholder.SourceID, &placeholder.Count)
	return placeholder, err
}

func GetSourceIDsForEntry(entry *Entry) []EntrySourcePlaceholder {
	results := query("select SourceID, Count from EntrySources where EntryID = ?", entry.ID()).rows(parseEntrySourcePlaceholder)

	sources := make([]EntrySourcePlaceholder, len(results))
	for i, result := range results {
		if id, ok := result.(EntrySourcePlaceholder); ok {
			sources[i] = id
		}
	}
	return sources
}

// ** Translations

type Translation struct {
	Entry        Entry
	Language     string
	Translation  string
	Translator   string
	IsPreferred  bool
	IsConflicted bool
}

func (translation *Translation) ID() uint64 {
	if translation == nil {
		return 0
	}

	var str = translation.Language + "  ---  " + translation.Translator
	return translation.Entry.ID() + hash64(str)
}

func parseTranslation(rows *sql.Rows) (Result, error) {
	t := Translation{}
	var entryID string
	err := rows.Scan(&entryID, &t.Language, &t.Translation, &t.Translator, &t.IsPreferred, &t.IsConflicted)
	if entry := GetEntryByID(entryID); entry == nil {
		return nil, nil
	} else {
		t.Entry = *entry
	}
	return t, err
}

const translationFields = "EntryID, Language, Translation, Translator, IsPreferred, IsConflicted"

func GetTranslations() []*Translation {
	results := query("select " + translationFields + " from Translations").rows(parseTranslation)
	translations := make([]*Translation, len(results))
	for i, result := range results {
		if translation, ok := result.(Translation); ok {
			translations[i] = &translation
		}
	}
	return translations
}

func GetTranslationByID(id string) *Translation {
	result := query("select "+translationFields+" from Translations where TranslationID = ?", id).row(parseTranslation)
	if translation, ok := result.(Translation); ok {
		return &translation
	}
	return nil
}

func GetTranslationsForLanguage(language string) []*Translation {
	results := query("select "+translationFields+" from Translations where Language = ?", language).rows(parseTranslation)
	translations := make([]*Translation, len(results))
	for i, result := range results {
		if translation, ok := result.(Translation); ok {
			translations[i] = &translation
		}
	}
	return translations
}

func (entry *Entry) GetTranslations(language string) []*Translation {
	results := query("select "+translationFields+" from Translations where EntryID = ? and Language = ?", entry.ID(), language).rows(parseTranslation)
	translations := make([]*Translation, len(results))
	for i, result := range results {
		if translation, ok := result.(Translation); ok {
			translations[i] = &translation
		}
	}
	return translations
}

func (entry *Entry) GetTranslationBy(language, translator string) *Translation {
	result := query("select "+translationFields+" from Translations where EntryID = ? and Language = ? and Translator = ?", entry.ID(), language, translator).row(parseTranslation)
	if translation, ok := result.(Translation); ok {
		return &translation
	}
	return nil
}

func (entry *Entry) GetMatchingTranslation(language, translation string) *Translation {
	result := query("select "+translationFields+" from Translations where EntryID = ? and Language = ? and Translation = ?", entry.ID(), language, translation).row(parseTranslation)
	if translation, ok := result.(Translation); ok {
		return &translation
	}
	return nil
}

func (translation *Translation) HasChanged() bool {
	underlying := translation.Entry.GetTranslationBy(translation.Language, translation.Translator)
	return underlying != nil && underlying.Translation == translation.Translation
}

func (translation *Translation) Save(clearVotes bool) {
	keyfields := map[string]interface{}{
		"TranslationID": translation.ID(),
	}
	fields := map[string]interface{}{
		"EntryID":     translation.Entry.ID(),
		"Language":    translation.Language,
		"Translator":  translation.Translator,
		"Translation": translation.Translation,
		"IsPreferred": translation.IsPreferred,
		"IsConflicted": translation.IsConflicted,
	}
	saveRecord("Translations", keyfields, fields)
	if clearVotes {
		ClearVotes(translation)
	}
}

// ** Votes

type Vote struct {
	Translation Translation
	Voter       *User
	Vote        bool
}

const voteFields = "TranslationID, Voter, Vote"

func parseVote(rows *sql.Rows) (Result, error) {
	v := Vote{}
	var translationID, voter string
	err := rows.Scan(&translationID, &voter, &v.Vote)
	if err != nil {
		return nil, err
	}

	if translation := GetTranslationByID(translationID); translation == nil {
		return nil, nil
	} else {
		v.Translation = *translation
	}

	v.Voter = GetUserByEmail(voter)
	return v, err
}

func (translation *Translation) GetVote(voter *User) *Vote {
	result := query("select " + voteFields + " from Votes").row(parseVote)
	if vote, ok := result.(Vote); ok {
		vote.Translation = *translation
		vote.Voter = voter
		return &vote
	}
	return nil
}

func (entry *Entry) GetTranslationVotes(language string) []*Vote {
	results := query("select "+voteFields+" from Votes where EntryID = ? and Language = ?", entry.ID(), language).rows(parseVote)
	votes := make([]*Vote, len(results))
	for i, result := range results {
		if vote, ok := result.(Vote); ok {
			votes[i] = &vote
		}
	}
	return votes
}

func (vote *Vote) Save() {
	keyfields := map[string]interface{}{
		"TranslationID": vote.Translation.ID(),
		"Voter":      vote.Voter.Email,
	}
	fields := map[string]interface{}{
		"Vote": vote.Vote,
	}
	saveRecord("Votes", keyfields, fields)
}

func DeleteVote(vote *Vote) {
	keyfields := map[string]interface{}{
		"TranslationID": vote.Translation.ID(),
		"Voter":      vote.Voter.Email,
	}
	deleteRecord("Votes", keyfields)
}

func ClearVotes(translation *Translation) {
	keyfields := map[string]interface{}{
		"TranslationID": translation.ID(),
	}
	deleteRecord("Votes", keyfields)
}

func ClearOtherVotes(translation *Translation) {
	keyfields := map[string]interface{}{
		"TranslationID": translation.ID(),
		"Vote":     true,
	}
	deleteRecord("Votes", keyfields)
}

/*
func AddTranslation(entry *Entry, language, translation string, translator *User) {
	keyfields := map[string]interface{}{
		"EntryID":   entry.ID(),
		"Language":      language,
		"Translator":    translator.Email,
	}
	fields := map[string]interface{}{
		"Translation": translation,
	}
	saveRecord("Translations", keyfields, fields)
}*/

// ** Users

type User struct {
	Email          string
	Password       string
	Secret         string
	Name           string
	IsAdmin        bool
	Language       string
	IsLanguageLead bool
}

func parseUser(rows *sql.Rows) (Result, error) {
	u := User{}
	err := rows.Scan(&u.Email, &u.Password, &u.Secret, &u.Name, &u.IsAdmin, &u.Language, &u.IsLanguageLead)
	u.Email = strings.ToLower(u.Email)
	return u, err
}

const userFields = "Email, Password, Secret, Name, IsAdmin, Language, IsLanguageLead"

func GetUsers() []*User {
	results := query("select " + userFields + " from Users order by IsAdmin desc, Language asc, Name asc").rows(parseUser)
	users := make([]*User, len(results))
	for i, result := range results {
		if user, ok := result.(User); ok {
			users[i] = &user
		}
	}
	return users
}

func GetUserByEmail(email string) *User {
	result := query("select "+userFields+" from Users where Email = ?", email).row(parseUser)
	if user, ok := result.(User); ok {
		return &user
	}
	return nil
}

func GetUsersByLanguage(language string) []*User {
	results := query("select "+userFields+" from Users where Language = ? order by IsLanguageLead desc, Name asc", language).rows(parseUser)
	users := make([]*User, len(results))
	for i, result := range results {
		if user, ok := result.(User); ok {
			users[i] = &user
		}
	}
	return users
}

func GetLanguageLead(language string) *User {
	result := query("select "+userFields+" from Users where Language = ? and IsLanguageLead = 1", language).row(parseUser)
	if result != nil {
		if user, ok := result.(User); ok {
			return &user
		}
	}

	// users := GetUsersByLanguage(language)
	// if len(users) > 0 {
	// 	return users[0]
	// }
	return nil
}

func (user *User) Save() bool {
	keyfields := map[string]interface{}{
		"Email": user.Email,
	}
	fields := map[string]interface{}{
		"Password":       user.Password,
		"Secret":         user.Secret,
		"Name":           user.Name,
		"IsAdmin":        user.IsAdmin,
		"Language":       user.Language,
		"IsLanguageLead": user.IsLanguageLead,
	}
	return saveRecord("Users", keyfields, fields)
}

func (user *User) Delete() {
	keyfields := map[string]interface{}{
		"Email": user.Email,
	}
	deleteRecord("Users", keyfields)
}

func (user *User) CountTranslations() map[string]int {
	counts := make(map[string]int, len(Languages))
	query("select Language, Count(*) from Translations where Translator = ? group by Language", user.Email).rows(func(rows *sql.Rows) (Result, error) {
		var language string
		var count int
		rows.Scan(&language, &count)
		counts[language] = count
		return nil, nil
	})
	return counts
}

// ** Comments

type Comment struct {
	Entry       Entry
	Language    string
	Commenter   string
	Comment     string
	CommentDate mysql.Timestamp
}
