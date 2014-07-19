package main

import (
    "bytes"
    "flag"
    "fmt"
    "io/ioutil"
    "jh/repeatgenome"
    "log"
    "os"
    "runtime/pprof"
    "time"
)

func lines(byteSlice []byte) [][]byte {
    var lines [][]byte = bytes.Split(byteSlice, []byte{'\n'})
    // drop the trailing newlines
    newline := []byte("\n")
    for lastLine := lines[len(lines)-1]; len(lines) > 0 && (len(lastLine) == 0 || bytes.Equal(lastLine, newline)); lastLine = lines[len(lines)-1] {
        lines = lines[:len(lines)-1]
    }
    return lines
}

func fileLines(filepath string) (err error, linesBytes [][]byte) {
    rawBytes, err := ioutil.ReadFile(filepath)
    if err != nil {
        return err, nil
    } else {
        return nil, lines(rawBytes)
    }
}

func main() {

    if len(os.Args) < 2 {
        fmt.Println("arg error - usage: ./minimize <flags> <reference genome dir>")
        os.Exit(1)
    }
    genomeName := os.Args[len(os.Args)-1]

    cpuProfile := flag.Bool("cpuprof", false, "write cpu profile to file <genomeName>.cpuprof")
    memProfile := flag.Bool("memprof", false, "write memory profile to <genomeName>.memprof")
    debug := flag.Bool("debug", false, "run and print debugging tests")
    noKraken := flag.Bool("no_kraken", false, "don't generate Kraken data structure")
    dontWriteMins := flag.Bool("no_write_kraken", false, "don't write the Kraken data to file")
    writeJSON := flag.Bool("json", false, "write JSON representation of class tree to <genomeName>.classtree.json")
    verifyClass := flag.Bool("verify_class", false, "run classification a second time, with SAM-formatted reads, to find percent correct classification")
    k_arg := flag.Uint("k", 31, "kmer length")
    m_arg := flag.Uint("m", 15, "minimizer length")
    flag.Parse()

    if *cpuProfile {
        fmt.Println("CPU profiler enabled")
        fmt.Println()
        os.Mkdir("profiles", os.ModeDir)
        f, err := os.Create("profiles/" + genomeName + ".cpuprof")
        if err != nil {
            panic(err)
        }
        pprof.StartCPUProfile(f)
        defer pprof.StopCPUProfile()
    }

    if *memProfile {
        fmt.Println("memory profiler enabled")
        fmt.Println()
    }

    if *debug {
        fmt.Println("debug tests enabled")
        fmt.Println()
    }

    genKraken := !(*noKraken)
    writeKraken := genKraken && !*dontWriteMins
    if !genKraken {
        fmt.Println("Kraken data structure disabled")
        fmt.Println()
    } else if !writeKraken {
        fmt.Println("Writing Kraken data to file disabled")
        fmt.Println()
    }


    if *writeJSON && genKraken {
        fmt.Println("class tree writeJSON write enabled")
        fmt.Println()
    } else if *writeJSON && !genKraken {
        fmt.Println("class tree writeJSON write enabled")
        fmt.Println("WARNING: you are writing the class tree writeJSON without minimizing - all node sizes will be 0")
        fmt.Println()
    }

    var k, m uint8
    if *k_arg > 255 || *m_arg > 255 {
        panic("k and m must be >= 255")
    } else {
        k = uint8(*k_arg)
        m = uint8(*m_arg)
        fmt.Println("k =", k)
        fmt.Println("m =", m)
        fmt.Println()
    }

    rgFlags := repeatgenome.Flags{*debug, *cpuProfile, *memProfile, genKraken, writeKraken, *writeJSON}
    err, rg := repeatgenome.Generate(genomeName, k, m, rgFlags)
    if err != nil {
        fmt.Println("./run-rg: RepeatGenome generation failed")
        panic(err)
    }

    fmt.Println(len(rg.Repeats), "repeats")
    fmt.Println(len(rg.ClassTree.ClassNodes), "class nodes")
    fmt.Println(len(rg.Matches), "matches")
    classNodesWithRepeats := 0
    for _, classNode := range rg.ClassTree.ClassNodes {
        if classNode.Repeat != nil {
            classNodesWithRepeats++
        }
    }
    fmt.Println(classNodesWithRepeats, "class nodes with repeats")
    fmt.Println()

    workingDirName, err := os.Getwd()
    if err != nil {
        log.Fatal(err)
    }
    readsDirName := workingDirName + "/" + genomeName + "-reads"
    currDir, err := os.Open(readsDirName)
    if err != nil {
        log.Fatal(err)
    }
    fileinfos, err := currDir.Readdir(-1)
    if err != nil {
        log.Fatal(err)
    }
    processedFiles := []os.FileInfo{}
    for _, fileinfo := range fileinfos {
        if len(fileinfo.Name()) > 5 && fileinfo.Name()[len(fileinfo.Name())-5 : ] == ".proc" {
            processedFiles = append(processedFiles, fileinfo)
        }
    }
    readsBytes := [][]byte{}
    for _, fileinfo := range processedFiles {
        _, theseReadsBytes := fileLines(readsDirName + "/" + fileinfo.Name())
        for _, bytesLine := range theseReadsBytes {
            readsBytes = append(readsBytes, bytesLine)
        }
    }

    startTime := time.Now()
    var numReads, numClassifiedReads uint64 = 0, 0
    for response := range rg.GetReadClassChan(readsBytes) {
        _, classNode := response.Read, response.ClassNode
        numReads++
        if classNode != nil {
            numClassifiedReads++
        }
    }
    netTime := time.Since(startTime)

    if numReads != uint64(len(readsBytes)) {
        panic("not all reads, or too many reads, returned from RepeatGenome.GetReadClassChan()")
    }

    if rg.Flags.Debug {
        classCount := make(map[*repeatgenome.ClassNode]uint64)
        for response := range rg.GetReadClassChan(readsBytes) {
            if response.ClassNode != nil {
                classCount[response.ClassNode]++
            }
        }

        for classNode, count := range classCount {
            fmt.Printf("%s\t%d\n", classNode.Name, count)
        }
    }

    fmt.Printf("%.2f thousand reads processed per minute\n", (float64(numReads) / 1000) / netTime.Minutes())
    fmt.Printf("RepeatGenome.Kmers comprises %.2f GB\n", rg.KmersGBSize())
    fmt.Printf("%.2f%% of the genome consists of repeat sequences\n", rg.PercentRepeats())
    fmt.Printf("%.2f%% of reads were classified with a repeat sequence (%d of %d)\n", 100 * (float64(numClassifiedReads) / float64(numReads)), numClassifiedReads, numReads)
    fmt.Println()

    if *verifyClass {
        fmt.Println("rerunning with SAM-formatted reads to check classification correctness")
        
        samFiles := []os.FileInfo{}
        for _, fileinfo := range fileinfos {
            if len(fileinfo.Name()) > 10 && fileinfo.Name()[len(fileinfo.Name())-10 : ] == ".fasta.sam" {
                samFiles = append(samFiles, fileinfo)
            }
        }
        readSAMs := []repeatgenome.ReadSAM{}
        for _, samFile := range samFiles {
            err, theseReadSAMs := repeatgenome.ParseReadSAMs(readsDirName + "/" + samFile.Name())
            if err != nil {
                panic(err)
            }
            for _, readSAM := range theseReadSAMs {
                readSAMs = append(readSAMs, readSAM)
            }
        }
        responses := []repeatgenome.ReadSAMResponse{}
        var numClassified = 0
        for response := range rg.GetReadSAMClassChan(readSAMs) {
            responses = append(responses, response)
            if response.ClassNode != nil {
                numClassified++
            }
        }

        if rg.Flags.Debug {
            for _, resp := range responses[:20] {
                fmt.Printf("%s[%d:%d]: %s\n", resp.ReadSAM.SeqName, resp.ReadSAM.StartInd, resp.ReadSAM.StartInd + uint64(len(resp.ReadSAM.Seq)), resp.ReadSAM.Seq)
                if resp.ClassNode != nil {
                    fmt.Printf("%s\n\n", resp.ClassNode.Name)
                } else {
                    fmt.Printf("<nil>\n\n")
                }
            }
            
            for _, resp := range responses {
                if resp.ReadSAM.SeqName == "chr2L" && resp.ReadSAM.StartInd == 84 {
                    fmt.Println("classified as", resp.ClassNode.Name)
                    repeatgenome.TestNodeSearch(resp.ClassNode, resp.ReadSAM)
                    fmt.Println()
                }
            }

            fmt.Println("Satellite/HETRP_DM instances:")
            for _, inst := range rg.RepeatMap["Satellite/HETRP_DM"].Instances {
                fmt.Printf("\t%s[%d:%d]\n", inst.SeqName, inst.SeqStart, inst.SeqEnd)
            }
            fmt.Println()
        }

        fmt.Println(numClassified, "of", len(responses), "reads classified")
        fmt.Printf("%.2f%% of classified reads overlapped an instance of their assigned repeat class\n", rg.PercentTrueClassifications(responses))
        fmt.Println()
    }

    fmt.Println(rg.Name, "successfully generated - exiting")
}
