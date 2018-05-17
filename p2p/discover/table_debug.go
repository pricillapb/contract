package discover

import (
	"bytes"
	"fmt"
)

func (tab *Table) Report() string {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "Table IPs:\n  %v\n", tab.ips)
	for i, b := range tab.buckets {
		if len(b.entries) == 0 {
			continue
		}
		fmt.Fprintf(buf, "Bucket %d:\n", i)
		fmt.Fprintf(buf, "  Nodes:\n")
		for _, e := range b.entries {
			fmt.Fprintf(buf, "     - %v\n", e)
		}
		if len(b.replacements) > 0 {
			fmt.Fprintf(buf, "  Replacements:\n")
			for _, e := range b.replacements {
				fmt.Fprintf(buf, "     - %v\n", e)
			}
		}
		fmt.Fprintf(buf, "  IPS:\n    %v\n", b.ips)
	}
	return buf.String()
}
