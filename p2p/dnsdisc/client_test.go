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
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestClientExample(t *testing.T) {
	r := newCountResolver(mapResolver{
		"n":                            {"enrtree-root=v1 hash=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtwE="},
		"TO4Q75OQ2N7DX4EOOR7X66A6OM.n": {"enrtree=F4YWVKW4N6B2DDZWFS4XCUQBHY,JTNOVTCP6XZUMXDRANXA6SWXTM,JGUFMSAGI7KZYB3P7IZW4S5Y3A"},
		"F4YWVKW4N6B2DDZWFS4XCUQBHY.n": {"enr=-H24QI0fqW39CMBZjJvV-EJZKyBYIoqvh69kfkF4X8DsJuXOZC6emn53SrrZD8P4v9Wp7NxgDYwtEUs3zQkxesaGc6UBgmlkgnY0gmlwhMsAcQGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOA=="},
		"JTNOVTCP6XZUMXDRANXA6SWXTM.n": {"enr=-H24QDquAsLj8mCMzJh8ka2BhVFg3n4V9efBJBiaXHcoL31vRJJef-lAseMhuQBEVpM_8Zrin0ReuUXJE7Fs8jy9FtwBgmlkgnY0gmlwhMYzZGOJc2VjcDI1NmsxoQLtfC0F55K2s1egRhrc6wWX5dOYjqla-OuKCELP92O3kA=="},
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": {"enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"},
	})
	c, _ := NewClient(r)
	spew.Dump(c.SyncTree("enrtree://AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@n"))
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
