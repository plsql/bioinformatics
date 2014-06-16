/*
    A barebones (at the moment) Go script for parsing and minimizing repeats

    The sole command line argument is the name of the reference genome (e.g. "dm3").

    This script expects there to be a subdirectory of the current directory named after the reference genome used (e.g. "dm3") that contains the following files:
        * a RepeatMasker library containing:
            - the match library (e.g. "dm3.fa.out")
            - the alignment information (e.g. "dm3.fa.align")
        * one or more reference genome files in FASTA format with the ".fa" filetype
*/


package main


import ("fmt"
        "log"
        "os"
        "io/ioutil"
        "strings"
        "strconv"
)

var err error

// uses 64-bit values, which is probably unnecessary for some fields
// this is done for simplicity (and strconv compatibility)
// it should probably be changed eventually
type Match struct {
    SW_Score int64
    PercDiv float64
    PercDel float64
    PercIns float64
    SeqName string
    SeqStart int64
    SeqEnd int64
    Left int64
    IsComplement bool
    RepeatType string
    RepeatClass string
    HangingBases int64
    RepeatStart int64
    RepeatEnd int64
    ID int64
}


// a minimal Match implementation for minimizing
type MatchBare struct {
    SeqName string
    SeqStart int64
    SeqEnd int64
    IsComplement bool
    RepeatType string
    RepeatClass string
    ID int64
}


type RefGenome struct {
    Name string
    // maps a chromosome name to a map of its sequences
    Chroms map[string](map[string]string)
}


func checkError(err error) {
    if err != nil {
        log.Fatal(err)
    }
}


func lines(str string) (numLines int, lines []string) {
    numLines = 0
    for i := 0; i < len(str); i++ {
        if str[i] == '\n' {
            numLines++
        }
    }
    lines = strings.Split(str, "\n")
    // drop the trailing newline's line if it's there
    if strings.TrimSpace(lines[len(lines)-1]) == "" {
        lines = lines[:len(lines)-1]
    }
    return numLines, lines
}

func parseMatches(matchLines []string) []Match {
    var matches []Match
    for i := 0; i < len(matchLines); i++ {
        rawVals := strings.Fields(matchLines[i])
        var match Match
        match.SW_Score, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.PercDiv, err = strconv.ParseFloat(rawVals[1], 64)
        checkError(err)
        match.PercDel, err = strconv.ParseFloat(rawVals[2], 64)
        checkError(err)
        match.PercIns, err = strconv.ParseFloat(rawVals[3], 64)
        checkError(err)
        match.SeqName = rawVals[4]
        match.SeqStart, err = strconv.ParseInt(rawVals[5], 10, 64)
        checkError(err)
        match.SeqEnd, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.Left, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.IsComplement = rawVals[8] == "C"
        match.RepeatType = rawVals[9]
        match.RepeatClass = rawVals[10]
        match.HangingBases, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.RepeatStart, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.RepeatEnd, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.ID, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)

        matches = append(matches, match)
    }
    return matches
}


func parseMatchBares(matchLines []string) []MatchBare {
    var matches []MatchBare
    for i := 0; i < len(matchLines); i++ {
        rawVals := strings.Fields(matchLines[i])
        var match MatchBare
        match.SeqName = rawVals[4]
        match.SeqStart, err = strconv.ParseInt(rawVals[5], 10, 64)
        checkError(err)
        match.SeqEnd, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)
        match.IsComplement = rawVals[8] == "C"
        match.RepeatType = rawVals[9]
        match.RepeatClass = rawVals[10]
        match.ID, err = strconv.ParseInt(rawVals[0], 10, 64)
        checkError(err)

        matches = append(matches, match)
    }
    return matches
}


func parseGenome(genomeName string) RefGenome {
    refGenome := RefGenome{genomeName, make(map[string](map[string]string))}
    chromFileInfos, err := ioutil.ReadDir(genomeName)
    checkError(err)
    // used below to store the two keys for RefGenome.Chroms
    for i := 0; i < len(chromFileInfos); i++ {
        chromFilename := strings.Join([]string{genomeName, chromFileInfos[i].Name()}, "/")
        // process the ref genome files (*.fa), not the repeat ref files (*.fa.out and *.fa.align)
        if strings.HasSuffix(chromFilename, ".fa") {
            rawSeqBytes, err := ioutil.ReadFile(chromFilename)
            checkError(err)
            rawSeq := string(rawSeqBytes)
            numLines, seqLines := lines(rawSeq)

            // ultimately contains this file's sequences
            seqMap := make(map[string]string)

            // populate thisSeq with the first seq's name
            thisSeq := append([]string(nil), strings.TrimSpace(seqLines[0])[1:])
            for i := 1; i < numLines; i++ {
                seqLine := strings.TrimSpace(seqLines[i])
                if seqLine[0] == byte('>') {
                    // we now have a full seq, and can write it to the map
                    seqMap[thisSeq[0]] = strings.Join(thisSeq[1:], "")
                    thisSeq = []string{seqLine[1:]}
                } else {
                    thisSeq = append(thisSeq, seqLine)
                }
            }
            // add the remaining sequence
            seqMap[thisSeq[0]] = strings.Join(thisSeq[1:], "")
            // finally, we insert this map into the full map
            chromName := chromFilename[len(genomeName)+1:len(chromFilename)-3]
            // must initialize the inner map
            refGenome.Chroms[chromName] = make(map[string]string)
            refGenome.Chroms[chromName] = seqMap
        }
    }
    return refGenome
}


func getMinimizer(kmer string, m int) int {
    currMin := 0
    for i := 0; i < len(kmer) - m + 1; i++ {
        if kmer[i:i+m] < kmer[currMin:currMin+m] {
            currMin = i
        }
    }
    return currMin
}


//func minimize(match 


func reverseComplement(seq string) string {
    key := map[byte]byte{'a':'t', 't':'a', 'c':'g', 'g':'c', 'A':'T', 'T':'A', 'C':'G', 'G':'C'}
    var revCompSeq []byte
    for i := 0; i < len(seq); i++ {
        revCompSeq = append(revCompSeq, key[seq[len(seq) - i - 1]])
    }
    return string(revCompSeq)
}


func main() {

    if len(os.Args) != 2 {
        fmt.Println(len(os.Args), "args supplied")
        fmt.Println("arg error - usage: ./minimize <reference genome dir>")
        os.Exit(1)
    }
    genomeName := os.Args[1]
    refGenome := parseGenome(genomeName)

    temp := strings.ToLower(refGenome.Chroms["chr2R"]["chr2R"])
    for i := range temp {
        ch := temp[i]
        if ch != byte('a') && ch != byte('t') && ch != byte('g') && ch != byte('c') {
            fmt.Printf("%d: %x\n", i, ch)
        }
    }
    // shows N's
    fmt.Println(temp[16668112:16668412])
        
    for k, v := range refGenome.Chroms {
        for k_, v_ := range v {
            fmt.Printf("refGenome.Chroms[%s][%s] = %s . . . %s\n", k, k_, v_[:10], v_[len(v_)-10:])
            fmt.Printf("len(refGenome.Chroms[%s][%s]) = %d\n", k, k_, len(v_))
            temp := strings.ToLower(v_)
            cnt := 0
            for i := range temp {
                if temp[i] == byte('a') || temp[i] == byte('t') || temp[i] == byte('g') || temp[i] == byte('c') {
                    cnt++
                }
            }
            fmt.Println("number of bases:", cnt, "\n")
        }
    }

    rawRepeatsBytes, err := ioutil.ReadFile("dm3/dm3.fa.out")
    checkError(err)
    rawRepeats := string(rawRepeatsBytes)

    numLines, matchLines := lines(rawRepeats)
    fmt.Println("number of repeat file lines:", numLines)
    // drop header
    matchLines = matchLines[3:]
    fmt.Println("number of parsed matchLines:", len(matchLines))

    matches := parseMatchBares(matchLines)

    fmt.Println("number of parsed matches", len(matches))
    fmt.Println("getMinimizer(\"ataggatcacgac\", 4) =", getMinimizer("ataggatcacgac", 4))
    fmt.Println("reverseComplement(\"aaAtGctACggT\") =", reverseComplement("aaAtGctACggT"))
}