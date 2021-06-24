package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/gogo/protobuf/proto"

	gcli "github.com/urfave/cli"

	messages "github.com/skycoin/hardware-wallet-protob/go"

	skyWallet "github.com/skycoin/hardware-wallet-go/src/skywallet"
)

func transactionSignCmd() gcli.Command {
	name := "transactionSign"
	return gcli.Command{
		Name:        name,
		Usage:       "Ask the device to sign a transaction using the provided information.",
		Description: "",
		Flags: []gcli.Flag{
			gcli.StringSliceFlag{
				Name:  "inputHash",
				Usage: "Hash of the Input of the transaction we expect the device to sign",
			},
			gcli.StringSliceFlag{
				Name:  "prevHash",
				Usage: "Hash of the previous transaction we expect the device to sign",
			},
			gcli.IntSliceFlag{
				Name:  "inputIndex",
				Usage: "Index of the input in the wallet",
			},
			gcli.StringSliceFlag{
				Name:  "outputAddress",
				Usage: "Addresses of the output for the transaction",
			},
			gcli.Int64SliceFlag{
				Name:  "coin",
				Usage: "Amount of coins",
			},
			gcli.Int64SliceFlag{
				Name:  "hour",
				Usage: "Number of hours",
			},
			gcli.IntSliceFlag{
				Name:  "addressIndex",
				Usage: "If the address is a return address tell its index in the wallet",
			},
			gcli.StringFlag{
				Name:   "deviceType",
				Usage:  "Device type to send instructions to, hardware wallet (USB) or emulator.",
				EnvVar: "DEVICE_TYPE",
			},
			gcli.StringFlag{
				Name:   "coinType",
				Value:  "SKY",
				Usage:  "Coin type to use on hardware-wallet.",
				EnvVar: "COIN_TYPE",
			},
		},
		OnUsageError: onCommandUsageError(name),
		Action: func(c *gcli.Context) {
			inputs := c.StringSlice("inputHash")
			prevHash := c.StringSlice("prevHash")
			inputIndex := c.IntSlice("inputIndex")
			outputs := c.StringSlice("outputAddress")
			coins := c.Int64Slice("coin")
			hours := c.Int64Slice("hour")
			addressIndex := c.IntSlice("addressIndex")
			coinType, err := skyWallet.CoinTypeFromString(c.String("coinType"))
			if err != nil {
				log.Error(err)
				return
			}
			if coinType != skyWallet.SkycoinCoinType && len(inputs) > 0 {
				log.Error(fmt.Errorf("coin type %s doesn't need input hash", coinType))
				return
			}

			if coinType != skyWallet.BitcoinCoinType && len(prevHash) > 0 {
				log.Error(fmt.Errorf("coin type %s doesn't need previous hash", coinType))
				return
			}

			device := skyWallet.NewDevice(skyWallet.DeviceTypeFromString(c.String("deviceType")))
			if device == nil {
				return
			}
			defer device.Close()

			if os.Getenv("AUTO_PRESS_BUTTONS") == "1" && device.Driver.DeviceType() == skyWallet.DeviceTypeEmulator && runtime.GOOS == "linux" {
				err := device.SetAutoPressButton(true, skyWallet.ButtonRight)
				if err != nil {
					log.Error(err)
					return
				}
			}

			if len(outputs) != len(coins) {
				fmt.Println("Every given output should have a coin value")
				return
			}

			switch coinType {
			case skyWallet.SkycoinCoinType:
				err = transactionSkycoinSign(device, inputs, outputs, coins, hours, inputIndex, addressIndex)
				if err != nil {
					log.Error(err)
				}
			case skyWallet.BitcoinCoinType:
				err = transactionBitcoinSign(device, prevHash, outputs, coins, inputIndex, addressIndex)
				if err != nil {
					log.Error(err)
				}
			default:
				log.Error(fmt.Errorf("TransactionSign is not implemented for %s yet", coinType))
			}
		},
	}
}

func transactionSkycoinSign(device *skyWallet.Device, inputs, outputs []string, coins, hours []int64, inputIndex, addressIndex []int) error {
	if len(inputs) != len(inputIndex) {
		return fmt.Errorf("every given input hash should have the an inputIndex")
	}
	if len(outputs) != len(hours) {
		return fmt.Errorf("every given output should have a coin value")
	}
	var transactionInputs []*messages.TxAck_TransactionType_TxInputType
	var transactionOutputs []*messages.TxAck_TransactionType_TxOutputType

	for i, input := range inputs {
		transactionInputs = append(transactionInputs, &messages.TxAck_TransactionType_TxInputType{
			AddressN: []uint32{*proto.Uint32(uint32((inputIndex)[i]))},
			HashIn:   proto.String(input),
		})
	}
	for i, output := range outputs {
		transactionOutputs = append(transactionOutputs, &messages.TxAck_TransactionType_TxOutputType{
			Address: proto.String(output),
			Coins:   proto.Uint64(uint64((coins)[i])),
			Hours:   proto.Uint64(uint64((hours)[i])),
		})
		if i < len(addressIndex) {
			transactionOutputs[len(transactionOutputs)-1].AddressN = []uint32{uint32((addressIndex)[i])}
		}
	}
	signer := skyWallet.SkycoinTransactionSigner{
		Inputs:   transactionInputs,
		Outputs:  transactionOutputs,
		Version:  1,
		LockTime: 0,
	}

	signatures, err := device.GeneralTransactionSign(&signer)
	if err != nil {
		return err
	}
	fmt.Println(signatures)
	return err
}

func transactionBitcoinSign(device *skyWallet.Device, prevHashes, outputs []string, coins []int64, inputIndex, addressIndex []int) error {
	if len(prevHashes) != len(inputIndex) {
		return fmt.Errorf("Every given input index should have a hash of previous the tx")
	}

	var transactionInputs []*messages.BitcoinTransactionInput
	var transactionOutputs []*messages.BitcoinTransactionOutput
	for i, prevHash := range prevHashes {
		var transactionInput messages.BitcoinTransactionInput
		transactionInput.AddressN = proto.Uint32(uint32(inputIndex[i]))
		decoded, err := hex.DecodeString(prevHash)
		if err != nil {
			return err
		}
		transactionInput.PrevHash = decoded
		transactionInputs = append(transactionInputs, &transactionInput)
	}
	for i, output := range outputs {
		var transactionOutput messages.BitcoinTransactionOutput
		transactionOutput.Address = proto.String(output)
		transactionOutput.Coin = proto.Uint64(uint64(coins[i]))
		if i < len(addressIndex) {
			transactionOutput.AddressIndex = proto.Uint32(uint32(addressIndex[i]))
		}
		transactionOutputs = append(transactionOutputs, &transactionOutput)
	}

	signer := skyWallet.BitcoinTransactionSigner{
		Inputs:   transactionInputs,
		Outputs:  transactionOutputs,
		Version:  1,
		LockTime: 0,
	}

	signatures, err := device.GeneralTransactionSign(&signer)
	if err != nil {
		return err
	}
	fmt.Println(signatures)
	return err
}
