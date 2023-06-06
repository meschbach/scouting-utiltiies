# Scouting Utilities
A set of utilities to analyze data and present data related to Scouting.

## Attendance Analysis
Takes a [Scoutbook](http://scoutbook.com/) attendance report to break it into Patrols with individual attendance
percentages.  Useful for understanding engagement.  Our Troop also moves Scouts out of Patrols if they are not active.

Example run `go build -o quickstart . && ./quickstart <spreadsheet-id>`

You'll need a `credentials.json` file containing your Google API credentials for an OAuth Desktop application.
