package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/meschbach/scouting-utilities/internal"
	"golang.org/x/exp/constraints"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	spreadsheetId := os.Args[1]
	suffix := time.Now().Second()
	uniqueName := fmt.Sprintf("raw-non-zero-%d", suffix)

	req := sheets.Request{
		DuplicateSheet: &sheets.DuplicateSheetRequest{
			NewSheetName:  uniqueName,
			SourceSheetId: 0,
		},
	}
	results, err := srv.Spreadsheets.BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{
		IncludeSpreadsheetInResponse: true,
		Requests:                     []*sheets.Request{&req},
	}).Do()
	if err != nil {
		log.Fatalf("Unable to duplicate sheets %v", err)
	}
	newSheetID := int64(-1)
	for _, sheet := range results.UpdatedSpreadsheet.Sheets {
		if sheet.Properties.Title == uniqueName {
			newSheetID = sheet.Properties.SheetId
		}
	}

	readRange := uniqueName + "!A:C"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return
	}

	var deleteIndexes []int
	fmt.Println("Events")
	for rowIndex, row := range resp.Values {
		if len(row) > 0 {
			if row[1] == "0%" || row[1] == "0.00%" {
				deleteIndexes = append(deleteIndexes, rowIndex)
			} else {
			}
		} else {
			//fmt.Printf("row %d ignored\n", rowIndex)
		}
	}
	deleteRowRequests := &sheets.BatchUpdateSpreadsheetRequest{}
	for deletionIndex, rowIndex := range deleteIndexes {
		targetRow := int64(rowIndex - deletionIndex)
		fmt.Printf("Deleting %d from original, %d adjusted\n", rowIndex, targetRow)
		deleteRowRequests.Requests = append(deleteRowRequests.Requests, &sheets.Request{
			DeleteDimension: newDeleteDimensionRequest(newSheetID, "ROWS", targetRow, targetRow+1),
		})
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetId, deleteRowRequests).Do()
	if err != nil {
		log.Fatalf("Unable to duplicate sheets %v", err)
	}

	//find 'Leader' in first row
	readRange = uniqueName + "!A1:ZZ3"
	resp, err = srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	//gsheets only returns len(row) of the last row with a value
	endDeletionColumn := maxOf(len(resp.Values[0]), len(resp.Values[1]), len(resp.Values[2]))
	startLeaderColumn := -1
	for leaderIndex, col := range resp.Values[0] {
		if col == "Leader" {
			fmt.Printf("Found 'Leader' starting at %d\n", leaderIndex)
			startLeaderColumn = leaderIndex
		}
	}

	if startLeaderColumn == -1 {
		log.Fatalf("Unable to find 'Leader' header in %#v\n", resp.Values[0])
	}

	deleteLeaderColumns := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&sheets.Request{
			DeleteDimension: newDeleteDimensionRequest(newSheetID, "COLUMNS", int64(startLeaderColumn), int64(endDeletionColumn)),
		}},
	}
	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetId, deleteLeaderColumns).Do()
	if err != nil {
		log.Fatalf("Unable to remove leader columns: %v", err)
	}

	//split by Patrol
	var allPatrols []*internal.PatrolRange
	var lastPatrol *internal.PatrolRange
	for columnIndex, header := range resp.Values[1] {
		if header != "" {
			if lastPatrol != nil {
				lastPatrol.End = int64(columnIndex - 1)
			}
			lastPatrol = &internal.PatrolRange{
				Patrol: strings.SplitN(header.(string), " ", 2)[1],
				Start:  int64(columnIndex),
				End:    -1,
			}
			allPatrols = append(allPatrols, lastPatrol)
		}
	}
	lastPatrol.End = int64(startLeaderColumn - 1)

	//create sheets
	createPatrolsBatch := &sheets.BatchUpdateSpreadsheetRequest{
		IncludeSpreadsheetInResponse: true,
	}
	for patrolIndex, p := range allPatrols {
		createPatrolsBatch.Requests = append(createPatrolsBatch.Requests, &sheets.Request{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{
					Title:   "filter-" + p.Patrol + "-" + fmt.Sprintf("%d", suffix),
					SheetId: int64(patrolIndex),
				},
			},
			//TODO figure out how to set our cell via sheet indexes
			//UpdateCells: &sheets.UpdateCellsRequest{
			//	Fields:          "",
			//	Range:           nil,
			//	Rows:            nil,
			//	Start:           nil,
			//	ForceSendFields: nil,
			//	NullFields:      nil,
			//},
		})
	}
	afterSheets, err := srv.Spreadsheets.BatchUpdate(spreadsheetId, createPatrolsBatch).Do()
	if err != nil {
		log.Fatalf("Unable to remove leader columns: %v", err)
	}

	headerValue := "=ARRAYFORMULA('" + uniqueName + "'!A2:C)"
	headerValues := &sheets.ValueRange{
		Values: [][]interface{}{[]interface{}{headerValue}},
	}

	batchCalculations := &sheets.BatchUpdateSpreadsheetRequest{}
	for _, p := range allPatrols {
		fmt.Printf("%s -- %d - %d\n", p.Patrol, p.Start, p.End)
		patrolValue := "=ARRAYFORMULA('" + uniqueName + "'!" + p.StartRange() + "2:" + p.EndRange() + ")"
		vr := &sheets.ValueRange{
			Values: [][]interface{}{[]interface{}{patrolValue}},
		}
		vr.MajorDimension = "COLUMNS"
		sheetName := "filter-" + p.Patrol + "-" + fmt.Sprintf("%d", suffix)

		_, err = srv.Spreadsheets.Values.Update(spreadsheetId, sheetName+"!A5:C99", headerValues).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Fatalf("unable to inject Patrol values: %v", err)
		}

		_, err = srv.Spreadsheets.Values.Update(spreadsheetId, sheetName+"!D5:ZZ99", vr).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Fatalf("unable to inject Patrol values: %v", err)
		}

		sheetID, found := sheetIndexByName(afterSheets.UpdatedSpreadsheet, sheetName)
		if !found {
			log.Fatalf("Could not find index of sheet %q", sheetName)
		}

		patrolColumnCount := p.ColumnCount()
		fmt.Printf("Patrol column count: %d\n", patrolColumnCount)
		batchCalculations.Requests = append(batchCalculations.Requests, repeatedRowCellRequest(sheetID, 0, 3, 3+patrolColumnCount, "=D3/D2", percent4PlacesFormat))
		batchCalculations.Requests = append(batchCalculations.Requests, repeatedRowCellRequest(sheetID, 1, 3, 3+patrolColumnCount, "=D3+D4", generalNumber))
		batchCalculations.Requests = append(batchCalculations.Requests, repeatedRowCellRequest(sheetID, 2, 3, 3+patrolColumnCount, fmt.Sprintf("=COUNTIF(D7:D999,%q&%q)", "=", "Yes"), generalNumber))
		batchCalculations.Requests = append(batchCalculations.Requests, repeatedRowCellRequest(sheetID, 3, 3, 3+patrolColumnCount, fmt.Sprintf("=COUNTIF(D7:D999,%q&%q)", "=", "No"), generalNumber))
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetId, batchCalculations).Do()
	if err != nil {
		log.Fatalf("Failed to update spreadsheets with calculations: %e", err)
	}
}

func newDeleteDimensionRequest(onSheetID int64, direction string, start int64, end int64) *sheets.DeleteDimensionRequest {
	return &sheets.DeleteDimensionRequest{
		Range: &sheets.DimensionRange{
			Dimension:  direction,
			EndIndex:   end,
			SheetId:    onSheetID,
			StartIndex: start,
		},
	}
}

func max[I constraints.Ordered](lhs I, rhs I) I {
	if lhs > rhs {
		return lhs
	}
	return rhs
}

func maxOf[I constraints.Ordered](first I, more ...I) I {
	highest := first
	for _, v := range more {
		highest = max(highest, v)
	}
	return highest
}

func fx[I any, O any](inputs []I, fx func(i I) O) []O {
	out := make([]O, len(inputs))
	for index, v := range inputs {
		out[index] = fx(v)
	}
	return out
}

func first[I any, O any](defaultValue O, inputs []I, filter func(i I) (O, bool)) (O, int, bool) {
	result := defaultValue
	index := -1
	found := false
	for i, v := range inputs {
		if output, matched := filter(v); matched {
			found = true
			index = i
			result = output
			break
		}
	}
	return result, index, found
}

var percent4PlacesFormat = &sheets.CellFormat{
	NumberFormat: &sheets.NumberFormat{
		Pattern: "##0.00%",
		Type:    "PERCENT",
	},
}
var generalNumber = &sheets.CellFormat{
	NumberFormat: &sheets.NumberFormat{
		Pattern: "####0",
		Type:    "NUMBER",
	},
}

func repeatCellRequest(sheetID, startingRow, startingColumn, endingRow, endingColumn int64, firstValue string, cellFormat *sheets.CellFormat) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Cell: &sheets.CellData{
				UserEnteredFormat: cellFormat,
				UserEnteredValue: &sheets.ExtendedValue{
					FormulaValue: &firstValue,
				},
			},
			Fields: "*",
			Range: &sheets.GridRange{
				EndColumnIndex:   endingColumn,
				EndRowIndex:      endingRow,
				SheetId:          sheetID,
				StartColumnIndex: startingColumn,
				StartRowIndex:    startingRow,
			},
		},
	}
}

func repeatedRowCellRequest(sheetID, row, firstColumn, lastColumn int64, firstValue string, cellFormat *sheets.CellFormat) *sheets.Request {
	return repeatCellRequest(sheetID, row, firstColumn, row+1, lastColumn, firstValue, cellFormat)
}

func sheetIndexByName(sheet *sheets.Spreadsheet, name string) (int64, bool) {
	id, _, found := first(-1, sheet.Sheets, func(i *sheets.Sheet) (int64, bool) {
		return i.Properties.SheetId, i.Properties.Title == name
	})
	return id, found
}
