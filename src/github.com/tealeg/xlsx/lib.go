package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

const (
	sheetEnding           = `</sheetData></worksheet>`
	fixedCellRefChar      = "$"
	cellRangeChar         = ":"
	externalSheetBangChar = "!"
)

// XLSXReaderError is the standard error type for otherwise undefined
// errors in the XSLX reading process.
type XLSXReaderError struct {
	Err string
}

// Error returns a string value from an XLSXReaderError struct in order
// that it might comply with the builtin.error interface.
func (e *XLSXReaderError) Error() string {
	return e.Err
}

// getRangeFromString is an internal helper function that converts
// XLSX internal range syntax to a pair of integers.  For example,
// the range string "1:3" yield the upper and lower integers 1 and 3.
func getRangeFromString(rangeString string) (lower int, upper int, error error) {
	var parts []string
	parts = strings.SplitN(rangeString, cellRangeChar, 2)
	if parts[0] == "" {
		error = errors.New(fmt.Sprintf("Invalid range '%s'\n", rangeString))
	}
	if parts[1] == "" {
		error = errors.New(fmt.Sprintf("Invalid range '%s'\n", rangeString))
	}
	lower, error = strconv.Atoi(parts[0])
	if error != nil {
		error = errors.New(fmt.Sprintf("Invalid range (not integer in lower bound) %s\n", rangeString))
	}
	upper, error = strconv.Atoi(parts[1])
	if error != nil {
		error = errors.New(fmt.Sprintf("Invalid range (not integer in upper bound) %s\n", rangeString))
	}
	return lower, upper, error
}

// ColLettersToIndex is used to convert a character based column
// reference to a zero based numeric column identifier.
func ColLettersToIndex(letters string) int {
	sum, mul, n := 0, 1, 0
	for i := len(letters) - 1; i >= 0; i, mul, n = i-1, mul*26, 1 {
		c := letters[i]
		switch {
		case 'A' <= c && c <= 'Z':
			n += int(c - 'A')
		case 'a' <= c && c <= 'z':
			n += int(c - 'a')
		}
		sum += n * mul
	}
	return sum
}

// Get the largestDenominator that is a multiple of a basedDenominator
// and fits at least once into a given numerator.
func getLargestDenominator(numerator, multiple, baseDenominator, power int) (int, int) {
	if numerator/multiple == 0 {
		return 1, power
	}
	next, nextPower := getLargestDenominator(
		numerator, multiple*baseDenominator, baseDenominator, power+1)
	if next > multiple {
		return next, nextPower
	}
	return multiple, power
}

// Convers a list of numbers representing a column into a alphabetic
// representation, as used in the spreadsheet.
func formatColumnName(colId []int) string {
	lastPart := len(colId) - 1

	var result strings.Builder
	for n, part := range colId {
		if n == lastPart {
			// The least significant number is in the
			// range 0-25, all other numbers are 1-26,
			// hence we use a differente offset for the
			// last part.
			result.WriteRune(rune(part + 65))
		} else {
			// Don't output leading 0s, as there is no
			// representation of 0 in this format.
			if part > 0 {
				result.WriteRune(rune(part + 64))
			}
		}
	}
	return result.String()
}

func smooshBase26Slice(b26 []int) []int {
	// Smoosh values together, eliminating 0s from all but the
	// least significant part.
	lastButOnePart := len(b26) - 2
	for i := lastButOnePart; i > 0; i-- {
		part := b26[i]
		if part == 0 {
			greaterPart := b26[i-1]
			if greaterPart > 0 {
				b26[i-1] = greaterPart - 1
				b26[i] = 26
			}
		}
	}
	return b26
}

func intToBase26(x int) (parts []int) {
	// Excel column codes are pure evil - in essence they're just
	// base26, but they don't represent the number 0.
	b26Denominator, _ := getLargestDenominator(x, 1, 26, 0)

	// This loop terminates because integer division of 1 / 26
	// returns 0.
	for d := b26Denominator; d > 0; d = d / 26 {
		value := x / d
		remainder := x % d
		parts = append(parts, value)
		x = remainder
	}
	return parts
}

// ColIndexToLetters is used to convert a zero based, numeric column
// indentifier into a character code.
func ColIndexToLetters(colRef int) string {
	parts := intToBase26(colRef)
	return formatColumnName(smooshBase26Slice(parts))
}

// RowIndexToString is used to convert a zero based, numeric row
// indentifier into its string representation.
func RowIndexToString(rowRef int) string {
	return strconv.Itoa(rowRef + 1)
}

// letterOnlyMapF is used in conjunction with strings.Map to return
// only the characters A-Z and a-z in a string
func letterOnlyMapF(rune rune) rune {
	switch {
	case 'A' <= rune && rune <= 'Z':
		return rune
	case 'a' <= rune && rune <= 'z':
		return rune - 32
	}
	return -1
}

// intOnlyMapF is used in conjunction with strings.Map to return only
// the numeric portions of a string.
func intOnlyMapF(rune rune) rune {
	if rune >= 48 && rune < 58 {
		return rune
	}
	return -1
}

// GetCoordsFromCellIDString returns the zero based cartesian
// coordinates from a cell name in Excel format, e.g. the cellIDString
// "A1" returns 0, 0 and the "B3" return 1, 2.
func GetCoordsFromCellIDString(cellIDString string) (x, y int, err error) {
	wrap := func(err error) (int, int, error) {
		return -1, -1, fmt.Errorf("GetCoordsFromCellIdString(%q): %w", cellIDString, err)
	}
	var letterPart string = strings.Map(letterOnlyMapF, cellIDString)
	y, err = strconv.Atoi(strings.Map(intOnlyMapF, cellIDString))
	if err != nil {
		return wrap(err)
	}
	y -= 1 // Zero based
	x = ColLettersToIndex(letterPart)
	return x, y, nil
}

// GetCellIDStringFromCoords returns the Excel format cell name that
// represents a pair of zero based cartesian coordinates.
func GetCellIDStringFromCoords(x, y int) string {
	return GetCellIDStringFromCoordsWithFixed(x, y, false, false)
}

// GetCellIDStringFromCoordsWithFixed returns the Excel format cell name that
// represents a pair of zero based cartesian coordinates.
// It can specify either value as fixed.
func GetCellIDStringFromCoordsWithFixed(x, y int, xFixed, yFixed bool) string {
	xStr := ColIndexToLetters(x)
	if xFixed {
		xStr = fixedCellRefChar + xStr
	}
	yStr := RowIndexToString(y)
	if yFixed {
		yStr = fixedCellRefChar + yStr
	}
	return xStr + yStr
}

// getMaxMinFromDimensionRef return the zero based cartesian maximum
// and minimum coordinates from the dimension reference embedded in a
// XLSX worksheet.  For example, the dimension reference "A1:B2"
// returns "0,0", "1,1".
func getMaxMinFromDimensionRef(ref string) (minx, miny, maxx, maxy int, err error) {
	var parts []string
	wrap := func(err error) (int, int, int, int, error) {
		return -1, -1, -1, -1, fmt.Errorf("getMaxMinFromDimensionRef: %w", err)
	}

	parts = strings.Split(ref, cellRangeChar)
	minx, miny, err = GetCoordsFromCellIDString(parts[0])
	if err != nil {
		return wrap(err)
	}
	maxx, maxy, err = GetCoordsFromCellIDString(parts[1])
	if err != nil {
		return wrap(err)
	}
	return
}

// calculateMaxMinFromWorkSheet works out the dimensions of a spreadsheet
// that doesn't have a DimensionRef set.  The only case currently
// known where this is true is with XLSX exported from Google Docs.
// This is also true for XLSX files created through the streaming APIs.
func calculateMaxMinFromWorksheet(worksheet *xlsxWorksheet) (minx, miny, maxx, maxy int, err error) {
	// Note, this method could be very slow for large spreadsheets.
	var x, y int
	var maxVal int

	wrap := func(err error) (int, int, int, int, error) {
		return -1, -1, -1, -1, fmt.Errorf("calculateMaxMinFromWorksheet: %w", err)
	}

	maxVal = int(^uint(0) >> 1)
	minx = maxVal
	miny = maxVal
	maxy = 0
	maxx = 0
	for _, row := range worksheet.SheetData.Row {
		for _, cell := range row.C {
			x, y, err = GetCoordsFromCellIDString(cell.R)
			if err != nil {
				return wrap(err)
			}
			if x < minx {
				minx = x
			}
			if x > maxx {
				maxx = x
			}
			if y < miny {
				miny = y
			}
			if y > maxy {
				maxy = y
			}
		}
	}
	if minx == maxVal {
		minx = 0
	}
	if miny == maxVal {
		miny = 0
	}
	return
}

// makeRowFromSpan will, when given a span expressed as a string,
// return an empty Row large enough to encompass that span and
// populate it with empty cells.  All rows start from cell 1 -
// regardless of the lower bound of the span.
func makeRowFromSpan(spans string, sheet *Sheet) *Row {
	var error error
	var upper int
	var row *Row

	row = new(Row)
	row.Sheet = sheet
	_, upper, error = getRangeFromString(spans)
	if error != nil {
		panic(error)
	}
	error = nil
	row.cellCount = upper
	row.cells = make([]*Cell, upper, upper)
	return row
}

// makeRowFromRaw returns the Row representation of the xlsxRow.
func makeRowFromRaw(rawrow xlsxRow, sheet *Sheet) *Row {
	var upper int
	var row *Row

	row = new(Row)
	row.Sheet = sheet
	upper = -1

	for _, rawcell := range rawrow.C {
		if rawcell.R != "" {
			x, _, error := GetCoordsFromCellIDString(rawcell.R)
			if error != nil {
				panic(fmt.Sprintf("Invalid Cell Coord, %s\n", rawcell.R))
			}
			if x > upper {
				upper = x
			}
			continue
		}
		upper++
	}
	upper++

	row.SetOutlineLevel(rawrow.OutlineLevel)
	row.cellCount = upper
	row.cells = make([]*Cell, upper, upper)
	return row
}

func makeEmptyRow(sheet *Sheet) *Row {
	row := new(Row)
	row.Sheet = sheet
	return row
}

type sharedFormula struct {
	x, y    int
	formula string
}

func formulaForCell(rawcell xlsxC, sharedFormulas map[int]sharedFormula) string {
	var res string

	f := rawcell.F
	if f == nil {
		return ""
	}
	if f.T == "shared" {
		x, y, err := GetCoordsFromCellIDString(rawcell.R)
		if err != nil {
			res = f.Content
		} else {
			if f.Ref != "" {
				res = f.Content
				sharedFormulas[f.Si] = sharedFormula{x, y, res}
			} else {
				sharedFormula := sharedFormulas[f.Si]
				dx := x - sharedFormula.x
				dy := y - sharedFormula.y
				orig := []byte(sharedFormula.formula)
				var start, end int
				var stringLiteral bool
				for end = 0; end < len(orig); end++ {
					c := orig[end]

					if c == '"' {
						stringLiteral = !stringLiteral
					}

					if stringLiteral {
						continue // Skip characters in quotes
					}

					if c >= 'A' && c <= 'Z' || c == '$' {
						res += string(orig[start:end])
						start = end
						end++
						foundNum := false
						for ; end < len(orig); end++ {
							idc := orig[end]
							if idc >= '0' && idc <= '9' || idc == '$' {
								foundNum = true
							} else if idc >= 'A' && idc <= 'Z' {
								if foundNum {
									break
								}
							} else {
								break
							}
						}
						if foundNum {
							cellID := string(orig[start:end])
							res += shiftCell(cellID, dx, dy)
							start = end
						}
					}
				}
				if start < len(orig) {
					res += string(orig[start:])
				}
			}
		}
	} else {
		res = f.Content
	}
	return strings.Trim(res, " \t\n\r")
}

// shiftCell returns the cell shifted according to dx and dy taking into consideration of absolute
// references with dollar sign ($)
func shiftCell(cellID string, dx, dy int) string {
	fx, fy, _ := GetCoordsFromCellIDString(cellID)

	// Is fixed column?
	fixedCol := strings.Index(cellID, fixedCellRefChar) == 0

	// Is fixed row?
	fixedRow := strings.LastIndex(cellID, fixedCellRefChar) > 0

	if !fixedCol {
		// Shift column
		fx += dx
	}

	if !fixedRow {
		// Shift row
		fy += dy
	}

	// New shifted cell
	shiftedCellID := GetCellIDStringFromCoords(fx, fy)

	if !fixedCol && !fixedRow {
		return shiftedCellID
	}

	// There are absolute references, need to put the $ back into the formula.
	letterPart := strings.Map(letterOnlyMapF, shiftedCellID)
	numberPart := strings.Map(intOnlyMapF, shiftedCellID)

	result := ""

	if fixedCol {
		result += "$"
	}

	result += letterPart

	if fixedRow {
		result += "$"
	}

	result += numberPart

	return result
}

// fillCellData attempts to extract a valid value, usable in
// CSV form from the raw cell value.  Note - this is not actually
// general enough - we should support retaining tabs and newlines.
func fillCellData(rawCell xlsxC, refTable *RefTable, sharedFormulas map[int]sharedFormula, cell *Cell) {
	val := strings.Trim(rawCell.V, " \t\n\r")
	cell.formula = formulaForCell(rawCell, sharedFormulas)
	switch rawCell.T {
	case "s": // Shared String
		cell.cellType = CellTypeString
		if val != "" {
			ref, err := strconv.Atoi(val)
			if err != nil {
				panic(err)
			}
			cell.Value, cell.RichText = refTable.ResolveSharedString(ref)
		}
	case "inlineStr":
		cell.cellType = CellTypeInline
		fillCellDataFromInlineString(rawCell, cell)
	case "b": // Boolean
		cell.Value = val
		cell.cellType = CellTypeBool
	case "e": // Error
		cell.Value = val
		cell.cellType = CellTypeError
	case "str":
		// String Formula (special type for cells with formulas that return a string value)
		// Unlike the other string cell types, the string is stored directly in the value.
		cell.Value = val
		cell.cellType = CellTypeStringFormula
	case "d": // Date: Cell contains a date in the ISO 8601 format.
		cell.Value = val
		cell.cellType = CellTypeDate
	case "": // Numeric is the default
		fallthrough
	case "n": // Numeric
		cell.Value = val
		cell.cellType = CellTypeNumeric
	default:
		panic(errors.New("invalid cell type"))
	}
}

// fillCellDataFromInlineString attempts to get inline string data and put it into a Cell.
func fillCellDataFromInlineString(rawcell xlsxC, cell *Cell) {
	cell.Value = ""
	cell.RichText = nil
	if rawcell.Is != nil {
		if rawcell.Is.T != nil {
			cell.Value = strings.Trim(rawcell.Is.T.getText(), " \t\n\r")
		} else {
			cell.RichText = xmlToRichText(rawcell.Is.R)
		}
	}
}

// readRowsFromSheet is an internal helper function that extracts the
// rows from a XSLXWorksheet, populates them with Cells and resolves
// the value references from the reference table and stores them in
// the rows and columns.
func readRowsFromSheet(Worksheet *xlsxWorksheet, file *File, sheet *Sheet, rowLimit int) error {
	var row *Row
	var maxCol, maxRow, colCount, rowCount int
	var reftable *RefTable
	var err error
	var insertRowIndex int // , insertColIndex int
	sharedFormulas := map[int]sharedFormula{}

	wrap := func(err error) error {
		return fmt.Errorf("readRowsFromSheet: %w", err)
	}
	if len(Worksheet.SheetData.Row) == 0 {
		sheet.MaxRow = 0
		sheet.MaxCol = 0
		return nil
	}
	reftable = file.referenceTable
	if len(Worksheet.Dimension.Ref) > 0 && len(strings.Split(Worksheet.Dimension.Ref, cellRangeChar)) == 2 && rowLimit == NoRowLimit {
		_, _, maxCol, maxRow, err = getMaxMinFromDimensionRef(Worksheet.Dimension.Ref)
	} else {
		_, _, maxCol, maxRow, err = calculateMaxMinFromWorksheet(Worksheet)
	}
	if err != nil {
		return wrap(err)
	}

	rowCount = maxRow + 1
	colCount = maxCol + 1

	if Worksheet.Cols != nil {
		// Columns can apply to a range, for convenience we expand the
		// ranges out into individual column definitions.
		for _, rawcol := range Worksheet.Cols.Col {
			col := &Col{
				Min:          rawcol.Min,
				Max:          rawcol.Max,
				Hidden:       rawcol.Hidden,
				Width:        rawcol.Width,
				OutlineLevel: rawcol.OutlineLevel,
				BestFit:      rawcol.BestFit,
				CustomWidth:  rawcol.CustomWidth,
				Phonetic:     rawcol.Phonetic,
				Collapsed:    rawcol.Collapsed,
			}
			if file.styles != nil {
				col.style = file.styles.getStyle(rawcol.Style)
				col.numFmt, col.parsedNumFmt = file.styles.getNumberFormat(rawcol.Style)
			}
			sheet.Cols.Add(col)
		}
	}

	for rowIndex := 0; rowIndex < len(Worksheet.SheetData.Row); rowIndex++ {
		rawrow := Worksheet.SheetData.Row[rowIndex]
		// range is not empty and only one range exist
		if len(rawrow.Spans) != 0 && strings.Count(rawrow.Spans, cellRangeChar) == 1 {
			row = makeRowFromSpan(rawrow.Spans, sheet)
		} else {
			row = makeRowFromRaw(rawrow, sheet)
		}
		// row.num = insertRowIndex
		row.num = rawrow.R - 1

		row.Hidden = rawrow.Hidden
		height, err := strconv.ParseFloat(rawrow.Ht, 64)
		if err == nil {
			row.SetHeight(height)
		}
		row.isCustom = rawrow.CustomHeight
		row.SetOutlineLevel(rawrow.OutlineLevel)

		for _, rawcell := range rawrow.C {
			if rawcell.R == "" {
				continue
			}
			h, v, err := Worksheet.MergeCells.getExtent(rawcell.R)
			if err != nil {
				return wrap(err)
			}
			x, _, err := GetCoordsFromCellIDString(rawcell.R)
			if err != nil {
				return wrap(err)
			}

			cellX := x

			cell := newCell(row, cellX)
			cell.HMerge = h
			cell.VMerge = v
			fillCellData(rawcell, reftable, sharedFormulas, cell)
			if file.styles != nil {
				cell.style = file.styles.getStyle(rawcell.S)
				cell.NumFmt, cell.parsedNumFmt = file.styles.getNumberFormat(rawcell.S)
			}
			cell.date1904 = file.Date1904
			// Cell is considered hidden if the row or the column of this cell is hidden
			col := sheet.Cols.FindColByIndex(cellX + 1)
			cell.Hidden = rawrow.Hidden || (col != nil && col.Hidden)
			row.cells[cellX] = cell
		}
		sheet.cellStore.WriteRow(row)

		insertRowIndex++
	}
	sheet.MaxRow = rowCount
	sheet.MaxCol = colCount

	if rowCount >= 0 {
		row, err = sheet.Row(0)
		if err != nil {
			return wrap(err)
		}
		sheet.setCurrentRow(row)
	}

	return nil
}

type indexedSheet struct {
	Index int
	Sheet *Sheet
	Error error
}

func readSheetViews(xSheetViews xlsxSheetViews) []SheetView {
	if xSheetViews.SheetView == nil || len(xSheetViews.SheetView) == 0 {
		return nil
	}
	sheetViews := []SheetView{}
	for _, xSheetView := range xSheetViews.SheetView {
		sheetView := SheetView{}
		if xSheetView.Pane != nil {
			xlsxPane := xSheetView.Pane
			pane := &Pane{}
			pane.XSplit = xlsxPane.XSplit
			pane.YSplit = xlsxPane.YSplit
			pane.TopLeftCell = xlsxPane.TopLeftCell
			pane.ActivePane = xlsxPane.ActivePane
			pane.State = xlsxPane.State
			sheetView.Pane = pane
		}
		sheetViews = append(sheetViews, sheetView)
	}
	return sheetViews
}

// readSheetFromFile is the logic of converting a xlsxSheet struct
// into a Sheet struct.  This work can be done in parallel and so
// readSheetsFromZipFile will spawn an instance of this function per
// sheet and get the results back on the provided channel.
func readSheetFromFile(rsheet xlsxSheet, fi *File, sheetXMLMap map[string]string, rowLimit int) (sheet *Sheet, errRes error) {

	wrap := func(err error) (*Sheet, error) {
		return nil, fmt.Errorf("readSheetFromFile: %w", err)
	}

	worksheet, err := getWorksheetFromSheet(rsheet, fi.worksheets, sheetXMLMap, rowLimit)
	if err != nil {
		return wrap(err)
	}

	sheet, err = NewSheetWithCellStore(rsheet.Name, fi.cellStoreConstructor)
	if err != nil {
		return wrap(err)
	}

	sheet.File = fi
	err = readRowsFromSheet(worksheet, fi, sheet, rowLimit)
	if err != nil {
		return wrap(err)
	}

	sheet.Hidden = rsheet.State == sheetStateHidden || rsheet.State == sheetStateVeryHidden
	sheet.SheetViews = readSheetViews(worksheet.SheetViews)
	if worksheet.AutoFilter != nil {
		autoFilterBounds := strings.Split(worksheet.AutoFilter.Ref, ":")
		sheet.AutoFilter = &AutoFilter{autoFilterBounds[0], autoFilterBounds[1]}
	}

	// Convert xlsxHyperlinks to Hyperlinks
	if worksheet.Hyperlinks != nil {

		worksheetRelsFile := fi.worksheetRels["sheet"+rsheet.SheetId]
		worksheetRels := new(xlsxWorksheetRels)
		rc, err := worksheetRelsFile.Open()
		if err != nil {
			return wrap(fmt.Errorf("file.Open: %w", err))
		}
		decoder := xml.NewDecoder(rc)
		err = decoder.Decode(worksheetRels)
		if err != nil {
			return wrap(fmt.Errorf("xml.Decoder.Decode: %w", err))
		}

		for _, xlsxLink := range worksheet.Hyperlinks.HyperLinks {
			newHyperLink := Hyperlink{}

			for _, rel := range worksheetRels.Relationships {
				if rel.Id == xlsxLink.RelationshipId {
					newHyperLink.Link = rel.Target
					break
				}
			}

			if xlsxLink.Tooltip != "" {
				newHyperLink.Tooltip = xlsxLink.Tooltip
			}
			if xlsxLink.DisplayString != "" {
				newHyperLink.DisplayString = xlsxLink.DisplayString
			}
			cellRef := xlsxLink.Reference
			x, y, err := GetCoordsFromCellIDString(cellRef)
			if err != nil {
				return wrap(err)
			}
			row, err := sheet.Row(y)
			if err != nil {
				return wrap(err)
			}
			cell := row.GetCell(x)
			cell.Hyperlink = newHyperLink
		}
	}

	sheet.SheetFormat.DefaultColWidth = worksheet.SheetFormatPr.DefaultColWidth
	sheet.SheetFormat.DefaultRowHeight = worksheet.SheetFormatPr.DefaultRowHeight
	sheet.SheetFormat.OutlineLevelCol = worksheet.SheetFormatPr.OutlineLevelCol
	sheet.SheetFormat.OutlineLevelRow = worksheet.SheetFormatPr.OutlineLevelRow
	if nil != worksheet.DataValidations {
		for _, dd := range worksheet.DataValidations.DataValidation {
			sheet.AddDataValidation(dd)
		}

	}

	return sheet, nil
}

// readSheetsFromZipFile is an internal helper function that loops
// over the Worksheets defined in the XSLXWorkbook and loads them into
// Sheet objects stored in the Sheets slice of a xlsx.File struct.
func readSheetsFromZipFile(f *zip.File, file *File, sheetXMLMap map[string]string, rowLimit int) (map[string]*Sheet, []*Sheet, error) {
	var workbook *xlsxWorkbook
	var err error
	var rc io.ReadCloser
	var decoder *xml.Decoder
	var sheetCount int

	wrap := func(err error) (map[string]*Sheet, []*Sheet, error) {
		return nil, nil, fmt.Errorf("readSheetsFromZipFile: %w", err)
	}

	workbook = new(xlsxWorkbook)
	rc, err = f.Open()
	if err != nil {
		return wrap(fmt.Errorf("file.Open: %w", err))
	}
	decoder = xml.NewDecoder(rc)
	err = decoder.Decode(workbook)
	if err != nil {
		return wrap(fmt.Errorf("xml.Decoder.Decode: %w", err))
	}
	file.Date1904 = workbook.WorkbookPr.Date1904

	for entryNum := range workbook.DefinedNames.DefinedName {
		file.DefinedNames = append(file.DefinedNames, &workbook.DefinedNames.DefinedName[entryNum])
	}

	// Only try and read sheets that have corresponding files.
	// Notably this excludes chartsheets don't right now
	var workbookSheets []xlsxSheet
	for _, sheet := range workbook.Sheets.Sheet {
		if f := worksheetFileForSheet(sheet, file.worksheets, sheetXMLMap); f != nil {
			workbookSheets = append(workbookSheets, sheet)
		}
	}
	sheetCount = len(workbookSheets)
	sheetsByName := make(map[string]*Sheet, sheetCount)
	sheets := make([]*Sheet, sheetCount)
	sheetChan := make(chan *indexedSheet, sheetCount)

	for i, rawsheet := range workbookSheets {
		i, rawsheet := i, rawsheet
		go func() {
			sheet, err := readSheetFromFile(rawsheet, file,
				sheetXMLMap, rowLimit)
			sheetChan <- &indexedSheet{
				Index: i,
				Sheet: sheet,
				Error: err,
			}
		}()
	}

	for j := 0; j < sheetCount; j++ {
		sheet := <-sheetChan
		if sheet.Error != nil {
			return wrap(sheet.Error)
		}
		sheetName := sheet.Sheet.Name
		sheetsByName[sheetName] = sheet.Sheet
		sheets[sheet.Index] = sheet.Sheet
	}
	return sheetsByName, sheets, nil
}

// readSharedStringsFromZipFile() is an internal helper function to
// extract a reference table from the sharedStrings.xml file within
// the XLSX zip file.
func readSharedStringsFromZipFile(f *zip.File) (*RefTable, error) {
	var sst *xlsxSST
	var err error
	var rc io.ReadCloser
	var decoder *xml.Decoder
	var reftable *RefTable

	wrap := func(err error) (*RefTable, error) {
		return nil, fmt.Errorf("readSharedStringsFromZipFile: %w", err)
	}

	// In a file with no strings it's possible that
	// sharedStrings.xml doesn't exist.  In this case the value
	// passed as f will be nil.
	if f == nil {
		return nil, nil
	}
	rc, err = f.Open()
	if err != nil {
		return wrap(err)
	}
	sst = new(xlsxSST)
	decoder = xml.NewDecoder(rc)
	err = decoder.Decode(sst)
	if err != nil {
		return wrap(err)
	}
	reftable = MakeSharedStringRefTable(sst)
	return reftable, nil
}

// readStylesFromZipFile() is an internal helper function to
// extract a style table from the style.xml file within
// the XLSX zip file.
func readStylesFromZipFile(f *zip.File, theme *theme) (*xlsxStyleSheet, error) {
	var style *xlsxStyleSheet
	var err error
	var rc io.ReadCloser
	var decoder *xml.Decoder

	wrap := func(err error) (*xlsxStyleSheet, error) {
		return nil, fmt.Errorf("readStylesFromZipFile: %w", err)
	}

	rc, err = f.Open()
	if err != nil {
		return wrap(err)
	}
	style = newXlsxStyleSheet(theme)
	decoder = xml.NewDecoder(rc)
	err = decoder.Decode(style)
	if err != nil {
		return wrap(err)
	}
	buildNumFmtRefTable(style)
	return style, nil
}

func buildNumFmtRefTable(style *xlsxStyleSheet) {
	if style.NumFmts != nil {
		for _, numFmt := range style.NumFmts.NumFmt {
			// We do this for the side effect of populating the NumFmtRefTable.
			style.addNumFmt(numFmt)
		}

	}
}

func readThemeFromZipFile(f *zip.File) (*theme, error) {
	wrap := func(err error) (*theme, error) {
		return nil, fmt.Errorf("readThemeFromZipFile: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return wrap(err)
	}

	var themeXml xlsxTheme
	err = xml.NewDecoder(rc).Decode(&themeXml)
	if err != nil {
		return wrap(err)
	}

	return newTheme(themeXml), nil
}

type WorkBookRels map[string]string

func (w *WorkBookRels) MakeXLSXWorkbookRels() xlsxWorkbookRels {
	relCount := len(*w)
	xWorkbookRels := xlsxWorkbookRels{}
	xWorkbookRels.Relationships = make([]xlsxWorkbookRelation, relCount+3)
	for k, v := range *w {
		index, err := strconv.Atoi(k[3:])
		if err != nil {
			panic(err.Error())
		}
		xWorkbookRels.Relationships[index-1] = xlsxWorkbookRelation{
			Id:     k,
			Target: v,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"}
	}

	relCount++
	sheetId := fmt.Sprintf("rId%d", relCount)
	xWorkbookRels.Relationships[relCount-1] = xlsxWorkbookRelation{
		Id:     sheetId,
		Target: "sharedStrings.xml",
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"}

	relCount++
	sheetId = fmt.Sprintf("rId%d", relCount)
	xWorkbookRels.Relationships[relCount-1] = xlsxWorkbookRelation{
		Id:     sheetId,
		Target: "theme/theme1.xml",
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"}

	relCount++
	sheetId = fmt.Sprintf("rId%d", relCount)
	xWorkbookRels.Relationships[relCount-1] = xlsxWorkbookRelation{
		Id:     sheetId,
		Target: "styles.xml",
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"}

	return xWorkbookRels
}

// readWorkbookRelationsFromZipFile is an internal helper function to
// extract a map of relationship ID strings to the name of the
// worksheet.xml file they refer to.  The resulting map can be used to
// reliably derefence the worksheets in the XLSX file.
func readWorkbookRelationsFromZipFile(workbookRels *zip.File) (WorkBookRels, error) {
	var sheetXMLMap WorkBookRels
	var wbRelationships *xlsxWorkbookRels
	var rc io.ReadCloser
	var decoder *xml.Decoder
	var err error

	wrap := func(err error) (WorkBookRels, error) {
		return nil, fmt.Errorf("readWorkbookRelationsFromZipFile :%w", err)
	}

	rc, err = workbookRels.Open()
	if err != nil {
		return wrap(err)
	}
	decoder = xml.NewDecoder(rc)
	wbRelationships = new(xlsxWorkbookRels)
	err = decoder.Decode(wbRelationships)
	if err != nil {
		return wrap(err)
	}
	sheetXMLMap = make(WorkBookRels)
	for _, rel := range wbRelationships.Relationships {
		if strings.HasSuffix(rel.Target, ".xml") && rel.Type == "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" {
			_, filename := path.Split(rel.Target)
			sheetXMLMap[rel.Id] = strings.Replace(filename, ".xml", "", 1)
		}
	}
	return sheetXMLMap, nil
}

// ReadZip() takes a pointer to a zip.ReadCloser and returns a
// xlsx.File struct populated with its contents.  In most cases
// ReadZip is not used directly, but is called internally by OpenFile.
func ReadZip(f *zip.ReadCloser, options ...FileOption) (*File, error) {
	defer f.Close()
	file, err := ReadZipReader(&f.Reader, options...)
	if err != nil {
		return nil, fmt.Errorf("ReadZip: %w", err)
	}
	return file, nil
}

// ReadZipReader() can be used to read an XLSX in memory without
// touching the filesystem.
func ReadZipReader(r *zip.Reader, options ...FileOption) (*File, error) {
	var err error
	var file *File
	var reftable *RefTable
	var sharedStrings *zip.File
	var sheetXMLMap map[string]string
	var sheetsByName map[string]*Sheet
	var sheets []*Sheet
	var style *xlsxStyleSheet
	var styles *zip.File
	var themeFile *zip.File
	var v *zip.File
	var workbook *zip.File
	var workbookRels *zip.File
	var worksheets map[string]*zip.File
	var worksheetRels map[string]*zip.File

	wrap := func(err error) (*File, error) {
		return nil, fmt.Errorf("ReadZipReader: %w", err)
	}

	file = NewFile(options...)
	worksheets = make(map[string]*zip.File, len(r.File))
	worksheetRels = make(map[string]*zip.File, len(r.File))
	for _, v = range r.File {
		switch v.Name {
		case "xl/sharedStrings.xml", `xl\sharedStrings.xml`:
			sharedStrings = v
		case "xl/workbook.xml", `xl\workbook.xml`:
			workbook = v
		case "xl/_rels/workbook.xml.rels", `xl\_rels\workbook.xml.rels`:
			workbookRels = v
		case "xl/styles.xml", `xl\styles.xml`:
			styles = v
		case "xl/theme/theme1.xml", `xl\theme\theme1.xml`:
			themeFile = v
		default:
			if len(v.Name) > 17 {
				if v.Name[0:13] == "xl/worksheets" || v.Name[0:13] == `xl\worksheets` {
					if v.Name[len(v.Name)-5:] == ".rels" {
						worksheetRels[v.Name[20:len(v.Name)-9]] = v
					} else {
						worksheets[v.Name[14:len(v.Name)-4]] = v
					}
				}
			}
		}
	}
	if workbookRels == nil {
		return wrap(fmt.Errorf("xl/_rels/workbook.xml.rels not found in input xlsx."))
	}
	sheetXMLMap, err = readWorkbookRelationsFromZipFile(workbookRels)
	if err != nil {
		return wrap(err)
	}
	if len(worksheets) == 0 {
		return wrap(fmt.Errorf("Input xlsx contains no worksheets."))
	}
	file.worksheets = worksheets
	file.worksheetRels = worksheetRels
	reftable, err = readSharedStringsFromZipFile(sharedStrings)
	if err != nil {
		return wrap(err)
	}
	file.referenceTable = reftable
	if themeFile != nil {
		theme, err := readThemeFromZipFile(themeFile)
		if err != nil {
			return wrap(err)
		}

		file.theme = theme
	}
	if styles != nil {
		style, err = readStylesFromZipFile(styles, file.theme)
		if err != nil {
			return wrap(err)
		}

		file.styles = style
	}
	sheetsByName, sheets, err = readSheetsFromZipFile(workbook, file, sheetXMLMap, file.rowLimit)
	if err != nil {
		return wrap(err)
	}
	if sheets == nil {
		readerErr := new(XLSXReaderError)
		readerErr.Err = "No sheets found in XLSX File"
		return wrap(readerErr)
	}
	file.Sheet = sheetsByName
	file.Sheets = sheets
	return file, nil
}

// truncateSheetXML will take in a reader to an XML sheet file and will return a reader that will read an equivalent
// XML sheet file with only the number of rows specified. This greatly speeds up XML unmarshalling when only
// a few rows need to be read from a large sheet.
// When sheets are truncated, all formatting present after the sheetData tag will be lost, but all of this formatting
// is related to printing and visibility, and is out of scope for most purposes of this library.
func truncateSheetXML(r io.Reader, rowLimit int) (io.Reader, error) {
	var rowCount int
	var token xml.Token
	var readErr error

	output := new(bytes.Buffer)
	r = io.TeeReader(r, output)
	decoder := xml.NewDecoder(r)

	for {
		token, readErr = decoder.Token()
		if readErr == io.EOF {
			break
		} else if readErr != nil {
			return nil, readErr
		}
		end, ok := token.(xml.EndElement)
		if ok && end.Name.Local == "row" {
			rowCount++
			if rowCount >= rowLimit {
				break
			}
		}
	}

	offset := decoder.InputOffset()
	output.Truncate(int(offset))

	if readErr != io.EOF {
		_, err := output.Write([]byte(sheetEnding))
		if err != nil {
			return nil, err
		}
	}
	return output, nil
}
