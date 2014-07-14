package repeatgenome

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "strings"
    "unsafe"
)

// used only for recursive JSON writing
type JSONNode struct {
    Name     string      `json:"name"`
    Size     uint64      `json:"size"`
    Children []*JSONNode `json:"children"`
}

func (repeatGenome *RepeatGenome) WriteClassJSON(useCumSize, printLeaves bool) {
    tree := &repeatGenome.ClassTree

    filename := repeatGenome.Name + ".classtree.json"
    outfile, err := os.Create(filename)
    checkError(err)
    defer outfile.Close()

    classToCount := make(map[uint16]uint64)
    for i := range repeatGenome.Kmers {
        lca_ID := *(*uint16)(unsafe.Pointer(&repeatGenome.Kmers[i][8]))
        classToCount[lca_ID]++
    }

    root := JSONNode{tree.Root.Name, classToCount[0], nil}
    tree.jsonRecPopulate(&root, classToCount)

    if useCumSize {
        root.jsonRecSize()
    }

    if !printLeaves {
        root.deleteLeaves()
    }

    jsonBytes, err := json.MarshalIndent(root, "", "\t")
    checkError(err)
    fmt.Fprint(outfile, string(jsonBytes))
}

func (classTree *ClassTree) jsonRecPopulate(jsonNode *JSONNode, classToCount map[uint16]uint64) {
    classNode := classTree.ClassNodes[jsonNode.Name]
    for i := range classNode.Children {
        child := new(JSONNode)
        child.Name = classNode.Children[i].Name
        child.Size = classToCount[classTree.ClassNodes[child.Name].ID]
        jsonNode.Children = append(jsonNode.Children, child)
        classTree.jsonRecPopulate(child, classToCount)
    }
}

func (jsonNode *JSONNode) jsonRecSize() uint64 {
    for i := range jsonNode.Children {
        jsonNode.Size += jsonNode.Children[i].jsonRecSize()
    }
    return jsonNode.Size
}

func (jsonNode *JSONNode) deleteLeaves() {
    branchChildren := []*JSONNode{}
    for i := range jsonNode.Children {
        child := jsonNode.Children[i]
        if child.Children != nil && len(child.Children) > 0 {
            branchChildren = append(branchChildren, child)
            child.deleteLeaves()
        }
    }
    jsonNode.Children = branchChildren
}

func (repeats *Repeats) Write(filename string) {
    outfile, err := os.Create(filename)
    checkError(err)
    defer outfile.Close()

    for i := range *repeats {
        if int((*repeats)[i].ID) == i {
            fmt.Fprintf(outfile, "%d %s\n", (*repeats)[i].ID, (*repeats)[i].Name)
        }
    }
}

func (refGenome *RepeatGenome) PrintChromInfo() {
    fmt.Println()
    for k, v := range refGenome.chroms {
        for k_, v_ := range v {
            fmt.Printf("refGenome.chroms[%s][%s] = %s . . . %s\n", k, k_, v_[:10], v_[len(v_)-10:])
            fmt.Printf("len(refGenome.chroms[%s][%s]) = %d\n", k, k_, len(v_))
        }
        fmt.Println()
    }
}

// a saner way of doing this would be to allocate a single k-long []byte and have a function populate it before printing
func (repeatGenome *RepeatGenome) WriteMins(minMap map[uint64]Kmers) error {
    k := repeatGenome.K
    m := repeatGenome.M
    kmerBuf := make([]byte, k, k)
    minBuf := make([]byte, m, m)
    filename := strings.Join([]string{repeatGenome.Name, ".mins"}, "")
    outfile, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer outfile.Close()
    writer := bufio.NewWriter(outfile)
    defer writer.Flush()

    var kmers Kmers
    var thisMin, kmerSeqInt uint64
    var lca_ID uint16

    for thisMin, kmers = range minMap {
        fillSeq(minBuf, thisMin)
        _, err = fmt.Fprintf(writer, ">%s\n", minBuf)
        if err != nil {
            return err
        }

        for i := range kmers {
            kmerSeqInt = *(*uint64)(unsafe.Pointer(&kmers[i][0]))
            lca_ID = *(*uint16)(unsafe.Pointer(&kmers[i][8]))
            fillSeq(kmerBuf, kmerSeqInt)
            _, err = fmt.Fprintf(writer, "\t%s %s\n", kmerBuf, repeatGenome.ClassTree.NodesByID[lca_ID].Name)
            if err != nil {
                return err
            }
        }
    }
    return nil
}

func printSeqInt(seqInt uint64, seqLen uint8) {
    var i uint8
    for i = 0; i < seqLen; i++ {
        // this tricky bit arithmetic shifts the two bits of interests to the two rightmost positions, then selects them with the and statement
        switch (seqInt >> (2 * (seqLen - i - 1))) & 3 {
        case 0:
            fmt.Print("a")
            break
        case 1:
            fmt.Print("c")
            break
        case 2:
            fmt.Print("g")
            break
        case 3:
            fmt.Print("t")
            break
        default:
            panic("error in printSeqInt() base selection")
        }
    }
}

func writeSeqInt(writer io.ByteWriter, seqInt uint64, seqLen uint8) error {
    var i uint8
    for i = 0; i < seqLen; i++ {
        // this tricky bit arithmetic shifts the two bits of interests to the two rightmost positions, then selects them with the and statement
        switch (seqInt >> (2 * (seqLen - i - 1))) & 3 {
        case 0:
            err = writer.WriteByte('a')
            break
        case 1:
            err = writer.WriteByte('c')
            break
        case 2:
            err = writer.WriteByte('g')
            break
        case 3:
            err = writer.WriteByte('t')
            break
        default:
            panic("error in printSeqInt() base selection")
        }
        if err != nil {
            return err
        }
    }
    return nil
}

func fillSeq(slice []byte, seqInt uint64) {
    if len(slice) > 32 {
        panic("slice of length greater than 32 passed to fillSeq()")
    }
    for i := range slice {
        switch (seqInt >> uint((2 * (len(slice) - i - 1)))) & 3 {
        case 0:
            slice[i] = byte('a')
            break
        case 1:
            slice[i] = byte('c')
            break
        case 2:
            slice[i] = byte('g')
            break
        case 3:
            slice[i] = byte('t')
            break
        default:
            panic("error in printSeqInt() base selection")
        }
    }
}
