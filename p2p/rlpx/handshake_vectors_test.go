// Copyright 2015 The go-ethereum Authors
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
package rlpx

import "crypto/ecdsa"

type handshakeTest struct {
	initiatorConfig, recipientConfig             *Config
	initiatorEphemeralKey, recipientEphemeralKey *ecdsa.PrivateKey
	initiatorNonce, recipientNonce               []byte

	// handshake packet ciphertexts.
	auth, authResp []byte

	// derived secrets
	initiatorEgressSecrets, initiatorIngressSecrets secrets
}

var handshakeTV = []handshakeTest{
	// initiator V4, recipient V5
	{
		initiatorConfig: &Config{Key: hexkey("5e173f6ac3c669587538e7727cf19b782a4f2fda07c1eaa662c593e5e85e3051"), ForceV4: true},
		recipientConfig: &Config{Key: hexkey("c45f950382d542169ea207959ee0220ec1491755abe405cd7498d6b16adb6df8")},

		initiatorEphemeralKey: hexkey("19c2185f4f40634926ebed3af09070ca9e029f2edd5fae6253074896205f5f6c"),
		recipientEphemeralKey: hexkey("d25688cf0ab10afa1a0e2dba7853ed5f1e5bf1c631757ed4e103b593ff3f5620"),
		initiatorNonce:        hexb("cd26fecb93657d1cd9e9eaf4f8be720b56dd1d39f190c4e1c6b7ec66f077bb11"),
		recipientNonce:        hexb("f37ec61d84cea03dcc5e8385db93248584e8af4b4d1c832d"),

		auth: hexb(`
			04a0274c5951e32132e7f088c9bdfdc76c9d91f0dc6078e848f8e3361193dbdc
			43b94351ea3d89e4ff33ddcefbc80070498824857f499656c4f79bbd97b6c51a
			514251d69fd1785ef8764bd1d262a883f780964cce6a14ff206daf1206aa073a
			2d35ce2697ebf3514225bef186631b2fd2316a4b7bcdefec8d75a1025ba2c540
			4a34e7795e1dd4bc01c6113ece07b0df13b69d3ba654a36e35e69ff9d482d88d
			2f0228e7d96fe11dccbb465a1831c7d4ad3a026924b182fc2bdfe016a6944312
			021da5cc459713b13b86a686cf34d6fe6615020e4acf26bf0d5b7579ba813e77
			23eb95b3cef9942f01a58bd61baee7c9bdd438956b426a4ffe238e61746a8c93
			d5e10680617c82e48d706ac4953f5e1c4c4f7d013c87d34a06626f498f34576d
			c017fdd3d581e83cfd26cf125b6d2bda1f1d56
		`),
		authResp: hexb(`
			049934a7b2d7f9af8fd9db941d9da281ac9381b5740e1f64f7092f3588d4f87f
			5ce55191a6653e5e80c1c5dd538169aa123e70dc6ffc5af1827e546c0e958e42
			dad355bcc1fcb9cdf2cf47ff524d2ad98cbf275e661bf4cf00960e74b5956b79
			9771334f426df007350b46049adb21a6e78ab1408d5e6ccde6fb5e69f0f4c92b
			b9c725c02f99fa72b9cdc8dd53cff089e0e73317f61cc5abf6152513cb7d833f
			09d2851603919bf0fbe44d79a09245c6e8338eb502083dc84b846f2fee1cc310
			d2cc8b1b9334728f97220bb799376233e113
		`),

		initiatorIngressSecrets: secrets{
			encKey: hexb("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  hexb("00000000000000000000000000000000"),
			macKey: hexb("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
		initiatorEgressSecrets: secrets{
			encKey: hexb("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  hexb("00000000000000000000000000000000"),
			macKey: hexb("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
	},

	// old V4 test vector from https://gist.github.com/fjl/3a78780d17c755d22df2
	{
		initiatorConfig: &Config{Key: hexkey("5e173f6ac3c669587538e7727cf19b782a4f2fda07c1eaa662c593e5e85e3051"), ForceV4: true},
		recipientConfig: &Config{Key: hexkey("c45f950382d542169ea207959ee0220ec1491755abe405cd7498d6b16adb6df8"), ForceV4: true},

		initiatorEphemeralKey: hexkey("19c2185f4f40634926ebed3af09070ca9e029f2edd5fae6253074896205f5f6c"),
		recipientEphemeralKey: hexkey("d25688cf0ab10afa1a0e2dba7853ed5f1e5bf1c631757ed4e103b593ff3f5620"),
		initiatorNonce:        hexb("cd26fecb93657d1cd9e9eaf4f8be720b56dd1d39f190c4e1c6b7ec66f077bb11"),
		recipientNonce:        hexb("f37ec61d84cea03dcc5e8385db93248584e8af4b4d1c832d8c7453c0089687a7"),

		auth: hexb(`
			04a0274c5951e32132e7f088c9bdfdc76c9d91f0dc6078e848f8e3361193dbdc
			43b94351ea3d89e4ff33ddcefbc80070498824857f499656c4f79bbd97b6c51a
			514251d69fd1785ef8764bd1d262a883f780964cce6a14ff206daf1206aa073a
			2d35ce2697ebf3514225bef186631b2fd2316a4b7bcdefec8d75a1025ba2c540
			4a34e7795e1dd4bc01c6113ece07b0df13b69d3ba654a36e35e69ff9d482d88d
			2f0228e7d96fe11dccbb465a1831c7d4ad3a026924b182fc2bdfe016a6944312
			021da5cc459713b13b86a686cf34d6fe6615020e4acf26bf0d5b7579ba813e77
			23eb95b3cef9942f01a58bd61baee7c9bdd438956b426a4ffe238e61746a8c93
			d5e10680617c82e48d706ac4953f5e1c4c4f7d013c87d34a06626f498f34576d
			c017fdd3d581e83cfd26cf125b6d2bda1f1d56
		`),
		authResp: hexb(`
			049934a7b2d7f9af8fd9db941d9da281ac9381b5740e1f64f7092f3588d4f87f
			5ce55191a6653e5e80c1c5dd538169aa123e70dc6ffc5af1827e546c0e958e42
			dad355bcc1fcb9cdf2cf47ff524d2ad98cbf275e661bf4cf00960e74b5956b79
			9771334f426df007350b46049adb21a6e78ab1408d5e6ccde6fb5e69f0f4c92b
			b9c725c02f99fa72b9cdc8dd53cff089e0e73317f61cc5abf6152513cb7d833f
			09d2851603919bf0fbe44d79a09245c6e8338eb502083dc84b846f2fee1cc310
			d2cc8b1b9334728f97220bb799376233e113
		`),

		initiatorIngressSecrets: secrets{
			encKey: hexb("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  hexb("00000000000000000000000000000000"),
			macKey: hexb("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
		initiatorEgressSecrets: secrets{
			encKey: hexb("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  hexb("00000000000000000000000000000000"),
			macKey: hexb("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
	},
}
