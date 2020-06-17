package consistentHash

import (
	"strconv"
	"strings"

	"github.com/g4zhuj/hashring"
)

func New(ipAndWeights []string) *hashring.HashRing {
	hashRing := hashring.NewHashRing(4000)

	nodeWeight := parseLogic(ipAndWeights)
	hashRing.AddNodes(nodeWeight)

	return hashRing
}

// eg:
// 192.168.1.11:3331,10
// 192.168.1.12:3332,10
// 192.168.1.13:3333,10
// 192.168.1.14:3334,10
func parseLogic(ipAndWeights []string) map[string]int {

	nodeWeight := map[string]int{}

	for _, v := range ipAndWeights {
		ss := strings.Split(v, ",")
		if len(ss) == 2 {
			weight, e := strconv.Atoi(ss[1])
			if e != nil {
				panic("gate list nil, strconv.Atoi error")
			}

			nodeWeight[ss[0]] = weight
		}
	}

	return nodeWeight
}
