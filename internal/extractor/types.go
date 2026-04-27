package extractor

type Book struct {
	Title    string
	Author   string
	Chapters []Chapter
}

type Chapter struct {
	Index int
	Title string
	Text  string
}
