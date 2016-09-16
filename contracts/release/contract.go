// This file is an automatically generated Go binding. Do not modify as any
// change will likely be lost upon the next re-generation!

package release

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ReleaseOracleABI is the input ABI used to generate the binding from.
const ReleaseOracleABI = `[{"constant":true,"inputs":[],"name":"proposedVersion","outputs":[{"name":"major","type":"uint32"},{"name":"minor","type":"uint32"},{"name":"patch","type":"uint32"},{"name":"commit","type":"bytes20"},{"name":"pass","type":"address[]"},{"name":"fail","type":"address[]"}],"type":"function"},{"constant":true,"inputs":[],"name":"signers","outputs":[{"name":"","type":"address[]"}],"type":"function"},{"constant":false,"inputs":[{"name":"user","type":"address"}],"name":"demote","outputs":[],"type":"function"},{"constant":true,"inputs":[{"name":"user","type":"address"}],"name":"authVotes","outputs":[{"name":"promote","type":"address[]"},{"name":"demote","type":"address[]"}],"type":"function"},{"constant":true,"inputs":[],"name":"currentVersion","outputs":[{"name":"major","type":"uint32"},{"name":"minor","type":"uint32"},{"name":"patch","type":"uint32"},{"name":"commit","type":"bytes20"},{"name":"time","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[],"name":"nuke","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"authProposals","outputs":[{"name":"","type":"address[]"}],"type":"function"},{"constant":false,"inputs":[{"name":"user","type":"address"}],"name":"promote","outputs":[],"type":"function"},{"constant":false,"inputs":[{"name":"major","type":"uint32"},{"name":"minor","type":"uint32"},{"name":"patch","type":"uint32"},{"name":"commit","type":"bytes20"}],"name":"release","outputs":[],"type":"function"},{"inputs":[{"name":"signers","type":"address[]"}],"type":"constructor"}]`

// ReleaseOracleBin is the compiled bytecode used for deploying new contracts.
const ReleaseOracleBin = "```@R`@Qa\x13S8\x03\x80a\x13S\x839\x81\x01`@R\x80Q\x01`\x00\x81Q`\x00\x14\x15a\x00\x84W`\x01`\xa0`\x02\n\x033\x16\x81R` \x81\x90R`@\x81 \x80T`\xff\x19\x16`\x01\x90\x81\x17\x90\x91U\x80T\x80\x82\x01\x80\x83U\x82\x81\x83\x80\x15\x82\x90\x11a\x00\xffW`\x00\x83\x81R` \x90 a\x00\xff\x91\x81\x01\x90\x83\x01[\x80\x82\x11\x15a\x01/W`\x00\x81U`\x01\x01a\x00pV[P`\x00[\x81Q\x81\x10\x15a\x01\x1fW`\x01`\x00`\x00P`\x00\x84\x84\x81Q\x81\x10\x15a\x00\x02W` \x90\x81\x02\x90\x91\x01\x81\x01Q`\x01`\xa0`\x02\n\x03\x16\x82R\x81\x01\x91\x90\x91R`@\x01`\x00 \x80T`\xff\x19\x16\x90\x91\x17\x90U`\x01\x80T\x80\x82\x01\x80\x83U\x82\x81\x83\x80\x15\x82\x90\x11a\x013W`\x00\x83\x81R` \x90 a\x013\x91\x81\x01\x90\x83\x01a\x00pV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x163\x17\x90U[PPa\x11߀a\x01t`\x009`\x00\xf3[P\x90V[PPP\x91\x90\x90`\x00R` `\x00 \x90\x01`\x00\x84\x84\x81Q\x81\x10\x15a\x00\x02WPPP` \x83\x81\x02\x85\x01\x01Q\x81T`\x01`\xa0`\x02\n\x03\x19\x16\x17\x90UP`\x01\x01a\x00\x88V```@R6\x15a\x00wW`\xe0`\x02\n`\x005\x04c&\xdbvH\x81\x14a\x00yW\x80cF\xf0\x97Z\x14a\x01\x9eW\x80c\\=\x00]\x14a\x02\nW\x80cd\xed1\xfe\x14a\x02\x93W\x80c\x9d\x88\x8e\x86\x14a\x03\x8dW\x80c\xbc\x8f\xbb\xf8\x14a\x03\xb2W\x80c\xbf\x8eϜ\x14a\x03\xfcW\x80c\xd0\xe0\x81:\x14a\x04hW\x80c\xd6|\xbe\xc9\x14a\x04yW[\x00[a\x04\x96`@\x80Q` \x81\x81\x01\x83R`\x00\x80\x83R\x83Q\x80\x83\x01\x85R\x81\x81R`\x04T`\x06\x80T\x87Q\x81\x87\x02\x81\x01\x87\x01\x90\x98R\x80\x88R\x93\x96\x87\x96\x87\x96\x87\x96\x91\x95\x94c\xff\xff\xff\xff\x81\x81\x16\x95d\x01\x00\x00\x00\x00\x83\x04\x82\x16\x95`@`\x02\n\x84\x04\x90\x92\x16\x94```\x02\n\x93\x84\x90\x04\x90\x93\x02\x93\x90\x92`\a\x92\x91\x84\x91\x90\x83\x01\x82\x82\x80\x15a\x01&W` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90[\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x01\aW[PPPPP\x91P\x80\x80T\x80` \x02` \x01`@Q\x90\x81\x01`@R\x80\x92\x91\x90\x81\x81R` \x01\x82\x80T\x80\x15a\x01\x83W` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90[\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x01dW[PPPPP\x90P\x95P\x95P\x95P\x95P\x95P\x95P\x90\x91\x92\x93\x94\x95V[`@\x80Q` \x81\x81\x01\x83R`\x00\x82R`\x01\x80T\x84Q\x81\x84\x02\x81\x01\x84\x01\x90\x95R\x80\x85Ra\x05X\x94\x92\x83\x01\x82\x82\x80\x15a\x01\xffW` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90[\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x01\xe0W[PPPPP\x90P[\x90V[a\x00w`\x045a\x06m\x81`\x00[`\x01`\xa0`\x02\n\x033\x16`\x00\x90\x81R` \x81\x90R`@\x81 T\x81\x90`\xff\x16\x15a\a\x00W`\x01`\xa0`\x02\n\x03\x84\x16\x81R`\x02` R`@\x81 \x91P[\x81T\x81\x10\x15a\a\x06W\x81T`\x01`\xa0`\x02\n\x033\x16\x90\x83\x90\x83\x90\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\aQWa\a\x00V[a\x05\xa2`\x045`@\x80Q` \x81\x81\x01\x83R`\x00\x80\x83R\x83Q\x80\x83\x01\x85R\x81\x81R`\x01`\xa0`\x02\n\x03\x86\x16\x82R`\x02\x83R\x90\x84\x90 \x80T\x85Q\x81\x85\x02\x81\x01\x85\x01\x90\x96R\x80\x86R\x93\x94\x91\x93\x90\x92`\x01\x84\x01\x92\x91\x84\x91\x83\x01\x82\x82\x80\x15a\x03 W` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90[\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x03\x01W[PPPPP\x91P\x80\x80T\x80` \x02` \x01`@Q\x90\x81\x01`@R\x80\x92\x91\x90\x81\x81R` \x01\x82\x80T\x80\x15a\x03}W` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90[\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x03^W[PPPPP\x90P\x91P\x91P\x91P\x91V[a\x06'`\x00`\x00`\x00`\x00`\x00`\x00`\b`\x00P\x80T\x90P`\x00\x14\x15a\x06pWa\x06\xf1V[a\x00wa\x06\xf9`\x00\x80\x80\x80\x80[`\x01`\xa0`\x02\n\x033\x16`\x00\x90\x81R` \x81\x90R`@\x81 T\x81\x90`\xff\x16\x15a\x11\xb6W\x82\x15\x80\x15a\x03\xf2WP`\x06T`\x00\x14[\x15a\f.Wa\x11\xb6V[`@\x80Q` \x81\x81\x01\x83R`\x00\x82R`\x03\x80T\x84Q\x81\x84\x02\x81\x01\x84\x01\x90\x95R\x80\x85Ra\x05X\x94\x92\x83\x01\x82\x82\x80\x15a\x01\xffW` \x02\x82\x01\x91\x90`\x00R` `\x00 \x90\x81T`\x01`\xa0`\x02\n\x03\x16\x81R`\x01\x91\x90\x91\x01\x90` \x01\x80\x83\x11a\x01\xe0W[PPPPP\x90Pa\x02\aV[a\x00w`\x045a\x06m\x81`\x01a\x02\x17V[a\x00w`\x045`$5`D5`d5a\a\x00\x84\x84\x84\x84`\x01a\x03\xbfV[`@Q\x80\x87c\xff\xff\xff\xff\x16\x81R` \x01\x86c\xff\xff\xff\xff\x16\x81R` \x01\x85c\xff\xff\xff\xff\x16\x81R` \x01\x84k\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16\x81R` \x01\x80` \x01\x80` \x01\x83\x81\x03\x83R\x85\x81\x81Q\x81R` \x01\x91P\x80Q\x90` \x01\x90` \x02\x80\x83\x83\x82\x90`\x00`\x04` \x84`\x1f\x01\x04`\x03\x02`\x0f\x01\xf1P\x90P\x01\x83\x81\x03\x82R\x84\x81\x81Q\x81R` \x01\x91P\x80Q\x90` \x01\x90` \x02\x80\x83\x83\x82\x90`\x00`\x04` \x84`\x1f\x01\x04`\x03\x02`\x0f\x01\xf1P\x90P\x01\x98PPPPPPPPP`@Q\x80\x91\x03\x90\xf3[`@Q\x80\x80` \x01\x82\x81\x03\x82R\x83\x81\x81Q\x81R` \x01\x91P\x80Q\x90` \x01\x90` \x02\x80\x83\x83\x82\x90`\x00`\x04` \x84`\x1f\x01\x04`\x03\x02`\x0f\x01\xf1P\x90P\x01\x92PPP`@Q\x80\x91\x03\x90\xf3[`@Q\x80\x80` \x01\x80` \x01\x83\x81\x03\x83R\x85\x81\x81Q\x81R` \x01\x91P\x80Q\x90` \x01\x90` \x02\x80\x83\x83\x82\x90`\x00`\x04` \x84`\x1f\x01\x04`\x03\x02`\x0f\x01\xf1P\x90P\x01\x83\x81\x03\x82R\x84\x81\x81Q\x81R` \x01\x91P\x80Q\x90` \x01\x90` \x02\x80\x83\x83\x82\x90`\x00`\x04` \x84`\x1f\x01\x04`\x03\x02`\x0f\x01\xf1P\x90P\x01\x94PPPPP`@Q\x80\x91\x03\x90\xf3[`@\x80Qc\xff\xff\xff\xff\x96\x87\x16\x81R\x94\x86\x16` \x86\x01R\x92\x90\x94\x16\x83\x83\x01Rk\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16``\x83\x01R`\x80\x82\x01\x92\x90\x92R\x90Q\x90\x81\x90\x03`\xa0\x01\x90\xf3[PV[`\b\x80T`\x00\x19\x81\x01\x90\x81\x10\x15a\x00\x02W`\x00\x91\x82R`\x04\x02\u007f\xf3\xf7\xa9\xfe6O\xaa\xb9;!m\xa5\n2\x14\x15O\"\xa0\xa2\xb4\x15\xb2:\x84\xc8\x16\x9e\x8bcn\xe3\x01\x90P\x80T`\x01\x82\x01Tc\xff\xff\xff\xff\x82\x81\x16\x99Pd\x01\x00\x00\x00\x00\x83\x04\x81\x16\x98P`@`\x02\n\x83\x04\x16\x96P```\x02\n\x91\x82\x90\x04\x90\x91\x02\x94Pg\xff\xff\xff\xff\xff\xff\xff\xff\x16\x92P\x90P[P\x90\x91\x92\x93\x94V[V[PPPP[PPPPV[P`\x00[`\x01\x82\x01T\x81\x10\x15a\aYW3`\x01`\xa0`\x02\n\x03\x16\x82`\x01\x01`\x00P\x82\x81T\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\a\xa3Wa\a\x00V[`\x01\x01a\x02RV[\x81T`\x00\x14\x80\x15a\anWP`\x01\x82\x01T`\x00\x14[\x15a\a\xcbW`\x03\x80T`\x01\x81\x01\x80\x83U\x82\x81\x83\x80\x15\x82\x90\x11a\a\xabW\x81\x83`\x00R` `\x00 \x91\x82\x01\x91\x01a\a\xab\x91\x90a\bQV[`\x01\x01a\a\nV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x16\x85\x17\x90U[\x82\x15a\biW\x81T`\x01\x81\x01\x80\x84U\x83\x91\x90\x82\x81\x83\x80\x15\x82\x90\x11a\b\x9eW`\x00\x83\x81R` \x90 a\b\x9e\x91\x81\x01\x90\x83\x01a\bQV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x16\x85\x17\x90U[`\x01`\xa0`\x02\n\x03\x84\x16`\x00\x90\x81R`\x02` \x90\x81R`@\x82 \x80T\x83\x82U\x81\x84R\x91\x83 \x90\x92\x91a\v/\x91\x90\x81\x01\x90[\x80\x82\x11\x15a\beW`\x00\x81U`\x01\x01a\bQV[P\x90V[\x81`\x01\x01`\x00P\x80T\x80`\x01\x01\x82\x81\x81T\x81\x83U\x81\x81\x15\x11a\tPW\x81\x83`\x00R` `\x00 \x91\x82\x01\x91\x01a\tP\x91\x90a\bQV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x163\x17\x90U`\x01T\x82T`\x02\x90\x91\x04\x90\x11a\b\xd2Wa\a\x00V[\x82\x80\x15a\b\xf8WP`\x01`\xa0`\x02\n\x03\x84\x16`\x00\x90\x81R` \x81\x90R`@\x90 T`\xff\x16\x15[\x15a\t\x87W`\x01`\xa0`\x02\n\x03\x84\x16`\x00\x90\x81R` \x81\x90R`@\x90 \x80T`\xff\x19\x16`\x01\x90\x81\x17\x90\x91U\x80T\x80\x82\x01\x80\x83U\x82\x81\x83\x80\x15\x82\x90\x11a\b\x00W\x81\x83`\x00R` `\x00 \x91\x82\x01\x91\x01a\b\x00\x91\x90a\bQV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x163\x17\x90U`\x01\x80T\x90\x83\x01T`\x02\x90\x91\x04\x90\x11a\b\xd2Wa\a\x00V[\x82\x15\x80\x15a\t\xadWP`\x01`\xa0`\x02\n\x03\x84\x16`\x00\x90\x81R` \x81\x90R`@\x90 T`\xff\x16[\x15a\b WP`\x01`\xa0`\x02\n\x03\x83\x16`\x00\x90\x81R` \x81\x90R`@\x81 \x80T`\xff\x19\x16\x90U[`\x01T\x81\x10\x15a\b W\x83`\x01`\xa0`\x02\n\x03\x16`\x01`\x00P\x82\x81T\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\n\xa3W`\x01\x80T`\x00\x19\x81\x01\x90\x81\x10\x15a\x00\x02W` `\x00\x90\x81 \x92\x90R`\x01\x80T\x92\x90\x91\x01T`\x01`\xa0`\x02\n\x03\x16\x91\x83\x90\x81\x10\x15a\x00\x02W\x90`\x00R` `\x00 \x90\x01`\x00a\x01\x00\n\x81T\x81`\x01`\xa0`\x02\n\x03\x02\x19\x16\x90\x83\x02\x17\x90UP`\x01`\x00P\x80T\x80\x91\x90`\x01\x90\x03\x90\x90\x81T\x81\x83U\x81\x81\x15\x11a\n\xabW`\x00\x83\x81R` \x90 a\n\xab\x91\x81\x01\x90\x83\x01a\bQV[`\x01\x01a\t\xd4V[PP`\x00`\x04\x81\x81U`\x05\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16\x90U`\x06\x80T\x83\x82U\x81\x84R\x91\x94P\x91\x92P\x82\x90a\v\x05\x90\u007f\xf6R\"#\x13\xe2\x84YR\x8d\x92\ve\x11\\\x16\xc0O>\xfc\x82\xaa\xed\xc9{\xe5\x9f?7|\r?\x90\x81\x01\x90a\bQV[P`\x01\x82\x01\x80T`\x00\x80\x83U\x91\x82R` \x90\x91 a\v%\x91\x81\x01\x90a\bQV[PPPPPa\b V[P`\x01\x82\x01\x80T`\x00\x80\x83U\x91\x82R` \x90\x91 a\vO\x91\x81\x01\x90a\bQV[P`\x00\x92PPP[`\x03T\x81\x10\x15a\a\x00W\x83`\x01`\xa0`\x02\n\x03\x16`\x03`\x00P\x82\x81T\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\f&W`\x03\x80T`\x00\x19\x81\x01\x90\x81\x10\x15a\x00\x02W` `\x00\x90\x81 \x92\x90R`\x03\x80T\x92\x90\x91\x01T`\x01`\xa0`\x02\n\x03\x16\x91\x83\x90\x81\x10\x15a\x00\x02W\x90`\x00R` `\x00 \x90\x01`\x00a\x01\x00\n\x81T\x81`\x01`\xa0`\x02\n\x03\x02\x19\x16\x90\x83\x02\x17\x90UP`\x03`\x00P\x80T\x80\x91\x90`\x01\x90\x03\x90\x90\x81T\x81\x83U\x81\x81\x15\x11a\x06\xfbW`\x00\x83\x81R` \x90 a\x06\xfb\x91\x81\x01\x90\x83\x01a\bQV[`\x01\x01a\vWV[`\x06T`\x00\x14\x15a\f\x8cW`\x04\x80Tc\xff\xff\xff\xff\x19\x16\x88\x17g\xff\xff\xff\xff\x00\x00\x00\x00\x19\x16d\x01\x00\x00\x00\x00\x88\x02\x17k\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x00\x00\x19\x16`@`\x02\n\x87\x02\x17k\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x16```\x02\n\x80\x87\x04\x02\x17\x90U[\x82\x80\x15a\r\bWP`\x04Tc\xff\xff\xff\xff\x88\x81\x16\x91\x16\x14\x15\x80a\f\xc1WP`\x04Tc\xff\xff\xff\xff\x87\x81\x16d\x01\x00\x00\x00\x00\x90\x92\x04\x16\x14\x15[\x80a\f\xdeWP`\x04Tc\xff\xff\xff\xff\x86\x81\x16`@`\x02\n\x90\x92\x04\x16\x14\x15[\x80a\r\bWP`\x04T```\x02\n\x90\x81\x90\x04\x02k\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x19\x90\x81\x16\x90\x85\x16\x14\x15[\x15a\r\x12Wa\x11\xb6V[P`\x06\x90P`\x00[\x81T\x81\x10\x15a\r[W\x81T`\x01`\xa0`\x02\n\x033\x16\x90\x83\x90\x83\x90\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\r\xa6Wa\x11\xb6V[P`\x00[`\x01\x82\x01T\x81\x10\x15a\r\xaeW3`\x01`\xa0`\x02\n\x03\x16\x82`\x01\x01`\x00P\x82\x81T\x81\x10\x15a\x00\x02W`\x00\x91\x82R` \x90\x91 \x01T`\x01`\xa0`\x02\n\x03\x16\x14\x15a\r\xe3Wa\x11\xb6V[`\x01\x01a\r\x1aV[\x82\x15a\r\xebW\x81T`\x01\x81\x01\x80\x84U\x83\x91\x90\x82\x81\x83\x80\x15\x82\x90\x11a\x0e W`\x00\x83\x81R` \x90 a\x0e \x91\x81\x01\x90\x83\x01a\bQV[`\x01\x01a\r_V[\x81`\x01\x01`\x00P\x80T\x80`\x01\x01\x82\x81\x81T\x81\x83U\x81\x81\x15\x11a\x0e\xa3W\x81\x83`\x00R` `\x00 \x91\x82\x01\x91\x01a\x0e\xa3\x91\x90a\bQV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x163\x17\x90U`\x01T\x82T`\x02\x90\x91\x04\x90\x11a\x0eTWa\x11\xb6V[\x82\x15a\x0e\xdaW`\x05\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16B\x17\x90U`\b\x80T`\x01\x81\x01\x80\x83U\x82\x81\x83\x80\x15\x82\x90\x11a\x0f/W`\x04\x02\x81`\x04\x02\x83`\x00R` `\x00 \x91\x82\x01\x91\x01a\x0f/\x91\x90a\x10HV[PPP`\x00\x92\x83RP` \x90\x91 \x01\x80T`\x01`\xa0`\x02\n\x03\x19\x163\x17\x90U`\x01\x80T\x90\x83\x01T`\x02\x90\x91\x04\x90\x11a\x0eTWa\x11\xb6V[`\x00`\x04\x81\x81U`\x05\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16\x90U`\x06\x80T\x83\x82U\x81\x84R\x91\x92\x91\x82\x90a\x11\xbf\x90\u007f\xf6R\"#\x13\xe2\x84YR\x8d\x92\ve\x11\\\x16\xc0O>\xfc\x82\xaa\xed\xc9{\xe5\x9f?7|\r?\x90\x81\x01\x90a\bQV[PPP\x91\x90\x90`\x00R` `\x00 \x90`\x04\x02\x01`\x00P`\x04\x80T\x82Tc\xff\xff\xff\xff\x19\x16c\xff\xff\xff\xff\x91\x82\x16\x17\x80\x84U\x82Td\x01\x00\x00\x00\x00\x90\x81\x90\x04\x83\x16\x02g\xff\xff\xff\xff\x00\x00\x00\x00\x19\x91\x90\x91\x16\x17\x80\x84U\x82T`@`\x02\n\x90\x81\x90\x04\x90\x92\x16\x90\x91\x02k\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x00\x00\x19\x91\x90\x91\x16\x17\x80\x83U\x81T```\x02\n\x90\x81\x90\x04\x81\x02\x81\x90\x04\x02k\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x91\x90\x91\x16\x17\x82U`\x05T`\x01\x83\x01\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16g\xff\xff\xff\xff\xff\xff\xff\xff\x90\x92\x16\x91\x90\x91\x17\x90U`\x06\x80T`\x02\x84\x01\x80T\x82\x82U`\x00\x82\x81R` \x90 \x94\x95\x94\x91\x92\x83\x92\x91\x82\x01\x91\x85\x82\x15a\x10\xa7W`\x00R` `\x00 \x91\x82\x01[\x82\x81\x11\x15a\x10\xa7W\x82T\x82U\x91`\x01\x01\x91\x90`\x01\x01\x90a\x10%V[PPPP`\x04\x01[\x80\x82\x11\x15a\beW`\x00\x80\x82U`\x01\x82\x01\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16\x90U`\x02\x82\x01\x80T\x82\x82U\x81\x83R` \x83 \x83\x91a\x10\x87\x91\x90\x81\x01\x90a\bQV[P`\x01\x82\x01\x80T`\x00\x80\x83U\x91\x82R` \x90\x91 a\x10@\x91\x81\x01\x90a\bQV[Pa\x10͒\x91P[\x80\x82\x11\x15a\beW\x80T`\x01`\xa0`\x02\n\x03\x19\x16\x81U`\x01\x01a\x10\xafV[PP`\x01\x81\x81\x01\x80T\x91\x84\x01\x80T\x80\x83U`\x00\x83\x81R` \x90 \x92\x93\x83\x01\x92\x90\x91\x82\x15a\x11\x1bW`\x00R` `\x00 \x91\x82\x01[\x82\x81\x11\x15a\x11\x1bW\x82T\x82U\x91`\x01\x01\x91\x90`\x01\x01\x90a\x11\x00V[Pa\x11'\x92\x91Pa\x10\xafV[PP`\x00`\x04\x81\x81U`\x05\x80Tg\xff\xff\xff\xff\xff\xff\xff\xff\x19\x16\x90U`\x06\x80T\x83\x82U\x81\x84R\x91\x97P\x91\x95P\x90\x93P\x84\x92Pa\x11\x86\x91P\u007f\xf6R\"#\x13\xe2\x84YR\x8d\x92\ve\x11\\\x16\xc0O>\xfc\x82\xaa\xed\xc9{\xe5\x9f?7|\r?\x90\x81\x01\x90a\bQV[P`\x01\x82\x01\x80T`\x00\x80\x83U\x91\x82R` \x90\x91 a\x11\xa6\x91\x81\x01\x90a\bQV[PPPPPa\x11\xb6V[PPPPP[PPPPPPPV[P`\x01\x82\x01\x80T`\x00\x80\x83U\x91\x82R` \x90\x91 a\x11\xb0\x91\x81\x01\x90a\bQV"

// DeployReleaseOracle deploys a new Ethereum contract, binding an instance of ReleaseOracle to it.
func DeployReleaseOracle(auth *bind.TransactOpts, backend bind.ContractBackend, signers []common.Address) (common.Address, *types.Transaction, *ReleaseOracle, error) {
	parsed, err := abi.JSON(strings.NewReader(ReleaseOracleABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, []byte(ReleaseOracleBin), backend, signers)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &ReleaseOracle{ReleaseOracleCaller: ReleaseOracleCaller{contract: contract}, ReleaseOracleTransactor: ReleaseOracleTransactor{contract: contract}}, nil
}

// ReleaseOracle is an auto generated Go binding around an Ethereum contract.
type ReleaseOracle struct {
	ReleaseOracleCaller     // Read-only binding to the contract
	ReleaseOracleTransactor // Write-only binding to the contract
}

// ReleaseOracleCaller is an auto generated read-only Go binding around an Ethereum contract.
type ReleaseOracleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReleaseOracleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ReleaseOracleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReleaseOracleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ReleaseOracleSession struct {
	Contract     *ReleaseOracle    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ReleaseOracleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ReleaseOracleCallerSession struct {
	Contract *ReleaseOracleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// ReleaseOracleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ReleaseOracleTransactorSession struct {
	Contract     *ReleaseOracleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// ReleaseOracleRaw is an auto generated low-level Go binding around an Ethereum contract.
type ReleaseOracleRaw struct {
	Contract *ReleaseOracle // Generic contract binding to access the raw methods on
}

// ReleaseOracleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ReleaseOracleCallerRaw struct {
	Contract *ReleaseOracleCaller // Generic read-only contract binding to access the raw methods on
}

// ReleaseOracleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ReleaseOracleTransactorRaw struct {
	Contract *ReleaseOracleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewReleaseOracle creates a new instance of ReleaseOracle, bound to a specific deployed contract.
func NewReleaseOracle(address common.Address, backend bind.ContractBackend) (*ReleaseOracle, error) {
	contract, err := bindReleaseOracle(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ReleaseOracle{ReleaseOracleCaller: ReleaseOracleCaller{contract: contract}, ReleaseOracleTransactor: ReleaseOracleTransactor{contract: contract}}, nil
}

// NewReleaseOracleCaller creates a new read-only instance of ReleaseOracle, bound to a specific deployed contract.
func NewReleaseOracleCaller(address common.Address, caller bind.ContractCaller) (*ReleaseOracleCaller, error) {
	contract, err := bindReleaseOracle(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &ReleaseOracleCaller{contract: contract}, nil
}

// NewReleaseOracleTransactor creates a new write-only instance of ReleaseOracle, bound to a specific deployed contract.
func NewReleaseOracleTransactor(address common.Address, transactor bind.ContractTransactor) (*ReleaseOracleTransactor, error) {
	contract, err := bindReleaseOracle(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &ReleaseOracleTransactor{contract: contract}, nil
}

// bindReleaseOracle binds a generic wrapper to an already deployed contract.
func bindReleaseOracle(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ReleaseOracleABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReleaseOracle *ReleaseOracleRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ReleaseOracle.Contract.ReleaseOracleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReleaseOracle *ReleaseOracleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.ReleaseOracleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReleaseOracle *ReleaseOracleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.ReleaseOracleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReleaseOracle *ReleaseOracleCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ReleaseOracle.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReleaseOracle *ReleaseOracleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReleaseOracle *ReleaseOracleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.contract.Transact(opts, method, params...)
}

// AuthProposals is a free data retrieval call binding the contract method 0xbf8ecf9c.
//
// Solidity: function authProposals() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleCaller) AuthProposals(opts *bind.CallOpts) ([]common.Address, error) {
	var (
		ret0 = new([]common.Address)
	)
	out := ret0
	err := _ReleaseOracle.contract.Call(opts, out, "authProposals")
	return *ret0, err
}

// AuthProposals is a free data retrieval call binding the contract method 0xbf8ecf9c.
//
// Solidity: function authProposals() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleSession) AuthProposals() ([]common.Address, error) {
	return _ReleaseOracle.Contract.AuthProposals(&_ReleaseOracle.CallOpts)
}

// AuthProposals is a free data retrieval call binding the contract method 0xbf8ecf9c.
//
// Solidity: function authProposals() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleCallerSession) AuthProposals() ([]common.Address, error) {
	return _ReleaseOracle.Contract.AuthProposals(&_ReleaseOracle.CallOpts)
}

// AuthVotes is a free data retrieval call binding the contract method 0x64ed31fe.
//
// Solidity: function authVotes(user address) constant returns(promote address[], demote address[])
func (_ReleaseOracle *ReleaseOracleCaller) AuthVotes(opts *bind.CallOpts, user common.Address) (struct {
	Promote []common.Address
	Demote  []common.Address
}, error) {
	ret := new(struct {
		Promote []common.Address
		Demote  []common.Address
	})
	out := ret
	err := _ReleaseOracle.contract.Call(opts, out, "authVotes", user)
	return *ret, err
}

// AuthVotes is a free data retrieval call binding the contract method 0x64ed31fe.
//
// Solidity: function authVotes(user address) constant returns(promote address[], demote address[])
func (_ReleaseOracle *ReleaseOracleSession) AuthVotes(user common.Address) (struct {
	Promote []common.Address
	Demote  []common.Address
}, error) {
	return _ReleaseOracle.Contract.AuthVotes(&_ReleaseOracle.CallOpts, user)
}

// AuthVotes is a free data retrieval call binding the contract method 0x64ed31fe.
//
// Solidity: function authVotes(user address) constant returns(promote address[], demote address[])
func (_ReleaseOracle *ReleaseOracleCallerSession) AuthVotes(user common.Address) (struct {
	Promote []common.Address
	Demote  []common.Address
}, error) {
	return _ReleaseOracle.Contract.AuthVotes(&_ReleaseOracle.CallOpts, user)
}

// CurrentVersion is a free data retrieval call binding the contract method 0x9d888e86.
//
// Solidity: function currentVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, time uint256)
func (_ReleaseOracle *ReleaseOracleCaller) CurrentVersion(opts *bind.CallOpts) (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Time   *big.Int
}, error) {
	ret := new(struct {
		Major  uint32
		Minor  uint32
		Patch  uint32
		Commit [20]byte
		Time   *big.Int
	})
	out := ret
	err := _ReleaseOracle.contract.Call(opts, out, "currentVersion")
	return *ret, err
}

// CurrentVersion is a free data retrieval call binding the contract method 0x9d888e86.
//
// Solidity: function currentVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, time uint256)
func (_ReleaseOracle *ReleaseOracleSession) CurrentVersion() (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Time   *big.Int
}, error) {
	return _ReleaseOracle.Contract.CurrentVersion(&_ReleaseOracle.CallOpts)
}

// CurrentVersion is a free data retrieval call binding the contract method 0x9d888e86.
//
// Solidity: function currentVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, time uint256)
func (_ReleaseOracle *ReleaseOracleCallerSession) CurrentVersion() (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Time   *big.Int
}, error) {
	return _ReleaseOracle.Contract.CurrentVersion(&_ReleaseOracle.CallOpts)
}

// ProposedVersion is a free data retrieval call binding the contract method 0x26db7648.
//
// Solidity: function proposedVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, pass address[], fail address[])
func (_ReleaseOracle *ReleaseOracleCaller) ProposedVersion(opts *bind.CallOpts) (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Pass   []common.Address
	Fail   []common.Address
}, error) {
	ret := new(struct {
		Major  uint32
		Minor  uint32
		Patch  uint32
		Commit [20]byte
		Pass   []common.Address
		Fail   []common.Address
	})
	out := ret
	err := _ReleaseOracle.contract.Call(opts, out, "proposedVersion")
	return *ret, err
}

// ProposedVersion is a free data retrieval call binding the contract method 0x26db7648.
//
// Solidity: function proposedVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, pass address[], fail address[])
func (_ReleaseOracle *ReleaseOracleSession) ProposedVersion() (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Pass   []common.Address
	Fail   []common.Address
}, error) {
	return _ReleaseOracle.Contract.ProposedVersion(&_ReleaseOracle.CallOpts)
}

// ProposedVersion is a free data retrieval call binding the contract method 0x26db7648.
//
// Solidity: function proposedVersion() constant returns(major uint32, minor uint32, patch uint32, commit bytes20, pass address[], fail address[])
func (_ReleaseOracle *ReleaseOracleCallerSession) ProposedVersion() (struct {
	Major  uint32
	Minor  uint32
	Patch  uint32
	Commit [20]byte
	Pass   []common.Address
	Fail   []common.Address
}, error) {
	return _ReleaseOracle.Contract.ProposedVersion(&_ReleaseOracle.CallOpts)
}

// Signers is a free data retrieval call binding the contract method 0x46f0975a.
//
// Solidity: function signers() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleCaller) Signers(opts *bind.CallOpts) ([]common.Address, error) {
	var (
		ret0 = new([]common.Address)
	)
	out := ret0
	err := _ReleaseOracle.contract.Call(opts, out, "signers")
	return *ret0, err
}

// Signers is a free data retrieval call binding the contract method 0x46f0975a.
//
// Solidity: function signers() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleSession) Signers() ([]common.Address, error) {
	return _ReleaseOracle.Contract.Signers(&_ReleaseOracle.CallOpts)
}

// Signers is a free data retrieval call binding the contract method 0x46f0975a.
//
// Solidity: function signers() constant returns(address[])
func (_ReleaseOracle *ReleaseOracleCallerSession) Signers() ([]common.Address, error) {
	return _ReleaseOracle.Contract.Signers(&_ReleaseOracle.CallOpts)
}

// Demote is a paid mutator transaction binding the contract method 0x5c3d005d.
//
// Solidity: function demote(user address) returns()
func (_ReleaseOracle *ReleaseOracleTransactor) Demote(opts *bind.TransactOpts, user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.contract.Transact(opts, "demote", user)
}

// Demote is a paid mutator transaction binding the contract method 0x5c3d005d.
//
// Solidity: function demote(user address) returns()
func (_ReleaseOracle *ReleaseOracleSession) Demote(user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Demote(&_ReleaseOracle.TransactOpts, user)
}

// Demote is a paid mutator transaction binding the contract method 0x5c3d005d.
//
// Solidity: function demote(user address) returns()
func (_ReleaseOracle *ReleaseOracleTransactorSession) Demote(user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Demote(&_ReleaseOracle.TransactOpts, user)
}

// Nuke is a paid mutator transaction binding the contract method 0xbc8fbbf8.
//
// Solidity: function nuke() returns()
func (_ReleaseOracle *ReleaseOracleTransactor) Nuke(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReleaseOracle.contract.Transact(opts, "nuke")
}

// Nuke is a paid mutator transaction binding the contract method 0xbc8fbbf8.
//
// Solidity: function nuke() returns()
func (_ReleaseOracle *ReleaseOracleSession) Nuke() (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Nuke(&_ReleaseOracle.TransactOpts)
}

// Nuke is a paid mutator transaction binding the contract method 0xbc8fbbf8.
//
// Solidity: function nuke() returns()
func (_ReleaseOracle *ReleaseOracleTransactorSession) Nuke() (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Nuke(&_ReleaseOracle.TransactOpts)
}

// Promote is a paid mutator transaction binding the contract method 0xd0e0813a.
//
// Solidity: function promote(user address) returns()
func (_ReleaseOracle *ReleaseOracleTransactor) Promote(opts *bind.TransactOpts, user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.contract.Transact(opts, "promote", user)
}

// Promote is a paid mutator transaction binding the contract method 0xd0e0813a.
//
// Solidity: function promote(user address) returns()
func (_ReleaseOracle *ReleaseOracleSession) Promote(user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Promote(&_ReleaseOracle.TransactOpts, user)
}

// Promote is a paid mutator transaction binding the contract method 0xd0e0813a.
//
// Solidity: function promote(user address) returns()
func (_ReleaseOracle *ReleaseOracleTransactorSession) Promote(user common.Address) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Promote(&_ReleaseOracle.TransactOpts, user)
}

// Release is a paid mutator transaction binding the contract method 0xd67cbec9.
//
// Solidity: function release(major uint32, minor uint32, patch uint32, commit bytes20) returns()
func (_ReleaseOracle *ReleaseOracleTransactor) Release(opts *bind.TransactOpts, major uint32, minor uint32, patch uint32, commit [20]byte) (*types.Transaction, error) {
	return _ReleaseOracle.contract.Transact(opts, "release", major, minor, patch, commit)
}

// Release is a paid mutator transaction binding the contract method 0xd67cbec9.
//
// Solidity: function release(major uint32, minor uint32, patch uint32, commit bytes20) returns()
func (_ReleaseOracle *ReleaseOracleSession) Release(major uint32, minor uint32, patch uint32, commit [20]byte) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Release(&_ReleaseOracle.TransactOpts, major, minor, patch, commit)
}

// Release is a paid mutator transaction binding the contract method 0xd67cbec9.
//
// Solidity: function release(major uint32, minor uint32, patch uint32, commit bytes20) returns()
func (_ReleaseOracle *ReleaseOracleTransactorSession) Release(major uint32, minor uint32, patch uint32, commit [20]byte) (*types.Transaction, error) {
	return _ReleaseOracle.Contract.Release(&_ReleaseOracle.TransactOpts, major, minor, patch, commit)
}
