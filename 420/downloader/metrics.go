// Copyright 2015 The The 420Integrated Development Group
// This file is part of the go-420coin library.
//
// The go-420coin library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-420coin library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-420coin library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/420integrated/go-420coin/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("420/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("420/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("420/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("420/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("420/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("420/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("420/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("420/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("420/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("420/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("420/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("420/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("420/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("420/downloader/states/drop", nil)

	throttleCounter = metrics.NewRegisteredCounter("420/downloader/throttle", nil)
)
