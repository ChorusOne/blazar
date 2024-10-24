package checks

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockSignedOneSigner(t *testing.T) {
	tests := []struct {
		name          string
		stateHeight   int
		signedBy      string
		expectedState CheckBlockStatus
	}{
		{
			name:          "Checked Too Late",
			stateHeight:   101,
			signedBy:      "DEADBEEF",
			expectedState: BlockSkipped,
		},
		{
			name:          "Did not sign yet",
			stateHeight:   100,
			signedBy:      "DEADBEEF",
			expectedState: BlockNotSignedYet,
		},
		{
			name:          "Signed",
			stateHeight:   100,
			signedBy:      "ABCDEF",
			expectedState: BlockSigned,
		},
	}

	nodeAddress := bytes.HexBytes{0xAB, 0xCD, 0xEF}
	var expectSignatureOnHeight int64 = 100

	for _, test := range tests {
		// response from client.GetConsensusState()
		consensusState := fmt.Sprintf(`{
      "height/round/step": "%d/0/1",
      "height_vote_set": [
        {
          "round": 0,
          "prevotes": [
            "nil-Vote",
            "Vote{43:%s 22107464/00/SIGNED_MSG_TYPE_PREVOTE(Prevote) 000000000000 8CB949D3858C 000000000000 @ 2024-09-09T12:25:51.227378426Z}"
          ]
        }
      ]
}`, test.stateHeight, test.signedBy)
		t.Run(test.name, func(t *testing.T) {
			state, err := CheckBlockSignedBy(nodeAddress, expectSignatureOnHeight, json.RawMessage(consensusState))
			require.NoError(t, err)
			assert.Equal(t, test.expectedState, state)
		})
	}
}
