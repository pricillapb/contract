// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package dnsdisc

import (
	"context"
	"crypto/ecdsa"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

func TestClientSyncTree(t *testing.T) {
	tree, err := MakeTree(testrecords, []string{"enrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	tree.Sign(testkeys[0], 3, "n")
	// fmt.Println(url)
	// for _, r := range tree.ToTXT("") {
	// 	fmt.Printf("%q: {%q}\n", r.Name+".n", r.Content)
	// }

	r := newCountResolver(mapResolver{
		"n":                            {"enrtree-root=v1 e=BWUSDYA7X5JTAPPV2TILKONCPE l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=3 sig=GW5Lf6NXO0PuCCsPuA-ND6TmAedDPDUmx08MXlX4uzUr2_3Rntx8KU4qQdlX2Uhrh02Vw4U3inRTqOsbQDIPuAE="},
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": {"enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"},
		"3KRT2RWDGBGOIT4BVUPTMREO7A.n": {"enr=-HW4QLZHjM4vZXkbp-5xJoHsKSbE7W39FPC8283X-y8oHcHPTnDDlIlzL5ArvDUlHZVDPgmFASrh7cWgLOLxj4wprRkHgmlkgnY0iXNlY3AyNTZrMaEC3t2jLMhDpCDX5mbSEwDn4L3iUfyXzoO8G28XvjGRkrA="},
		"RXN7EWFAAL6Y6XOWO3EPO4UKMM.n": {"enr=-HW4QPl99oxKbLdy5R-wcLCEz4DjqXKwhMPIdGDdrmm0VVt-Qh55V6jAVKIwWpJapx7UGX6Hzf80fnTr2lFunBb7mJCAgmlkgnY0iXNlY3AyNTZrMaEDymNMrg1JrLQB2KTGtv6MVbcNEVv0AHacwUAPMljNMTg="},
		"ZUPSLMLVE2CES5RRD4EN3JOQ4Y.n": {"enr=-HW4QOB8FyVDBZKT999sReSor1q_GfESyCa9_uTIpg-kYQtzFXer4AKiofJ3xkno6SlgBkmGAiXQY_Wb2YaioNdlqTEJgmlkgnY0iXNlY3AyNTZrMaECRZa6PJwtwsioWg0NO0w_-QkXb3prxepUZwpdxjXxnls="},
		"XMIEKCIYV4JLZRCKLKVRWYD4TU.n": {"enr=-HW4QORbKGbyp-BRQOwVCLD2OIiFh4-w79SWyfIA11OeRMdnBVGhiPgORBU3RE8xoHtSkMCI-z0REtnq91yU5hrnVvQGgmlkgnY0iXNlY3AyNTZrMaECqZUhm4jv86fjzvtmcqXkG-54fg3z_pzl2FnqnckqSmU="},
		"ZNCH5ZNIR5Y4RIORAE5WIRDT4Q.n": {"enr=-HW4QAqTJiybxoWIfDRnuxsC6xwmuY79J0aKaEBDz8mAx0Y8CxrOIMiHtHobA7SC_0I5aKN_Xy89hRJyzWSFYN5PyY4DgmlkgnY0iXNlY3AyNTZrMaEDOlFBdkZvqBXtSB_60JEQotNE9sm3jB0Ur8NRw6Ub4z0="},
		"BWUSDYA7X5JTAPPV2TILKONCPE.n": {"enrtree=N7ZW6LHUPSH4YQYAFWIG44M2OU,3KRT2RWDGBGOIT4BVUPTMREO7A,RXN7EWFAAL6Y6XOWO3EPO4UKMM,5VTS2SZK6TC3UZCOX3YIGNZ76I,ZUPSLMLVE2CES5RRD4EN3JOQ4Y,TP7N4KP2OBQJU2QPSVI7V5226E,6ENOKQN2BOKNIFFQW56UPMFC7A,NKPAZPIMU5YADTIXWWYVICDVQI,XMIEKCIYV4JLZRCKLKVRWYD4TU,ZNCH5ZNIR5Y4RIORAE5WIRDT4Q"},
		"N7ZW6LHUPSH4YQYAFWIG44M2OU.n": {"enr=-HW4QCmjL_ZHKP_UOCVv2avoj_tMgQAktxnCdJfx-HpeTOeDDi4EyKMnJLccLGlg5GyRo3wc4NUrJBmByM0bzZPHeHMIgmlkgnY0iXNlY3AyNTZrMaEDUybYcILt0ULt3FUY-tJEjOpWwm8-Bqf2EoBr17JOmKk="},
		"5VTS2SZK6TC3UZCOX3YIGNZ76I.n": {"enr=-HW4QNPOHkqXzibYBvK1rwT5t15o2IkphtInmWwsLCpMWzmyZopJ09CMAfTcCqzTNlw0ByaZB_A1yQHNsGMh-SmwfnwEgmlkgnY0iXNlY3AyNTZrMaECxUT5ee0C-7zsA9FRx8yK7C8M8vkJ07tWHWUeqEZ7DBY="},
		"TP7N4KP2OBQJU2QPSVI7V5226E.n": {"enr=-HW4QDVAsOFVAC3hCT2nGU4Wsu8g7m4tDTtyF87qgEna3FqyYyk4wYX6ZEjMZ0PBsk7E1IOggI3DbN1m_7bsUFpL0n0FgmlkgnY0iXNlY3AyNTZrMaECXYexc-eTwo7jInOJhAx-4oHocDZUpFghLuYpVqZn0J8="},
		"6ENOKQN2BOKNIFFQW56UPMFC7A.n": {"enr=-HW4QO7fsG9CUu_f9SPwjFx8pKL7Huyu5oULkdKNg5rpZndpD-Y_QIIb9AwOx1MoclwQoPWb47EbjmeLOXJq0CnZmF8CgmlkgnY0iXNlY3AyNTZrMaED_aHP9nTJDJoZdTn-PftTCGrOZPg-18bqvsdB9_OBzIA="},
		"NKPAZPIMU5YADTIXWWYVICDVQI.n": {"enr=-HW4QHihi1ZXeISPgoEfNZ1X8Y9MJnG5lEe2ahcfxLlY4GS2YJwW26w8UltTCGN64jwqdQJKccqohlzU3sS8qLIU8OIBgmlkgnY0iXNlY3AyNTZrMaEC7XwtBeeStrNXoEYa3OsFl-XTmI6pWvjrighCz_djt5A="},
	})
	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	stree, err := c.SyncTree("enrtree://APFGGTFOBVE2ZNAB3CSMNNX6RRK3ODIRLP2AA5U4YFAA6MSYZUYTQ@n")
	if err != nil {
		t.Fatal("sync error:", err)
	}
	if !reflect.DeepEqual(stree, tree) {
		t.Error("incomplete tree synced")
	}
}

type countResolver struct {
	qcount int
	child  Resolver
}

func newCountResolver(r Resolver) *countResolver {
	return &countResolver{child: r}
}

func (cr *countResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	cr.qcount++
	return cr.child.LookupTXT(ctx, name)
}

type mapResolver map[string][]string

func (mr mapResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return mr[name], nil
}

var (
	testkeys    []*ecdsa.PrivateKey
	testrecords []*enode.Node
)

func init() {
	testkeys = []*ecdsa.PrivateKey{
		hexkey("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"),
		hexkey("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a"),
		hexkey("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee"),
		hexkey("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"),
		hexkey("7018732ded552337dfbe3d6f7393b5fc2c4dff57d3420d16c91309a8bb47b51d"),
		hexkey("90725056af7cdbe7ba4ea470a5542c9b1e615b175ac18294e20bfad060f8cf5e"),
		hexkey("09ffd992a55877c2260e80eff793f229ed55983420532ea64d8473f87f1a981a"),
		hexkey("f51f6be7a7367376f5200ddc525ffb5835068b92c863ab92460cb31d107724a6"),
		hexkey("b300e6a49e8425bb3eda749ace0db408d43adcda3c5a808bf3abe86e059dee28"),
		hexkey("78e766d81cfa4e5f5d77b9804b6389b219b09967c392bbdf7f11c7df2931d313"),
	}
	testrecords = make([]*enode.Node, len(testkeys))
	for i, k := range testkeys {
		record := new(enr.Record)
		record.SetSeq(uint64(i))
		enode.SignV4(record, k)
		n, _ := enode.New(enode.ValidSchemes, record)
		testrecords[i] = n
	}
}

func hexkey(s string) *ecdsa.PrivateKey {
	k, err := crypto.HexToECDSA(s)
	if err != nil {
		panic("invalid private key " + s)
	}
	return k
}
