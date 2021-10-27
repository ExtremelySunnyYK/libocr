package evmreportcodec

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/smartcontractkit/libocr/offchainreporting2/reportingplugin/median"
	"github.com/smartcontractkit/libocr/offchainreporting2/types"
)

var reportTypes = getReportTypes()

func getReportTypes() abi.Arguments {
	mustNewType := func(t string) abi.Type {
		result, err := abi.NewType(t, "", []abi.ArgumentMarshaling{})
		if err != nil {
			panic(fmt.Sprintf("Unexpected error during abi.NewType: %s", err))
		}
		return result
	}
	return abi.Arguments([]abi.Argument{
		{Name: "observationsTimestamp", Type: mustNewType("uint32")},
		{Name: "rawObservers", Type: mustNewType("bytes32")},
		{Name: "observations", Type: mustNewType("int192[]")},
		{Name: "juelsPerEth", Type: mustNewType("int192")},
	})
}

var _ median.ReportCodec = ReportCodec{}

type ReportCodec struct{}

func (ReportCodec) BuildReport(paos []median.ParsedAttributedObservation) (types.Report, error) {
	if len(paos) == 0 {
		return nil, fmt.Errorf("cannot build report from empty attributed observations")
	}

	// copy so we can safely re-order subsequently
	paos = append([]median.ParsedAttributedObservation{}, paos...)

	// get median timestamp
	sort.Slice(paos, func(i, j int) bool {
		return paos[i].Timestamp < paos[j].Timestamp
	})
	timestamp := paos[len(paos)/2].Timestamp

	// get median juelsPerEth
	sort.Slice(paos, func(i, j int) bool {
		return paos[i].JuelsPerEth.Cmp(paos[j].JuelsPerEth) < 0
	})
	juelsPerEth := paos[len(paos)/2].JuelsPerEth

	// sort by values
	sort.Slice(paos, func(i, j int) bool {
		return paos[i].Value.Cmp(paos[j].Value) < 0
	})

	observers := [32]byte{}
	observations := []*big.Int{}

	for i, pao := range paos {
		observers[i] = byte(pao.Observer)
		observations = append(observations, pao.Value)
	}

	reportBytes, err := reportTypes.Pack(timestamp, observers, observations, juelsPerEth)
	return types.Report(reportBytes), err
}

func (ReportCodec) MedianFromReport(report types.Report) (*big.Int, error) {
	reportElems := map[string]interface{}{}
	if err := reportTypes.UnpackIntoMap(reportElems, report); err != nil {
		return nil, fmt.Errorf("error during unpack: %w", err)
	}

	observationsIface, ok := reportElems["observations"]
	if !ok {
		return nil, fmt.Errorf("unpacked report has no 'observations'")
	}

	observations, ok := observationsIface.([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("cannot cast observations to []*big.Int, type is %T", observationsIface)
	}

	if len(observations) == 0 {
		return nil, fmt.Errorf("observations are empty")
	}

	median := observations[len(observations)/2]
	if median == nil {
		return nil, fmt.Errorf("median is nil")
	}

	return median, nil
}