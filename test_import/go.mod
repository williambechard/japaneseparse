module test_import

go 1.23.0

replace github.com/williambechard/japaneseparse => ../

require github.com/williambechard/japaneseparse v0.0.0-00010101000000-000000000000

require (
	github.com/ikawaha/kagome-dict v1.1.6 // indirect
	github.com/ikawaha/kagome-dict/ipa v1.2.5 // indirect
	github.com/ikawaha/kagome/v2 v2.10.2 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
