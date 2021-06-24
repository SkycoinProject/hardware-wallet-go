package cli

import (
	"fmt"
	"os"
	"runtime"

	messages "github.com/skycoin/hardware-wallet-protob/go"

	gcli "github.com/urfave/cli"

	skyWallet "github.com/skycoin/hardware-wallet-go/src/skywallet"
)

func addressGenCmd() gcli.Command {
	name := "addressGen"
	return gcli.Command{
		Name:        name,
		Usage:       "Generate skycoin addresses using the firmware",
		Description: "",
		Flags: []gcli.Flag{
			gcli.IntFlag{
				Name:  "addressN",
				Value: 1,
				Usage: "Number of addresses to generate. Assume 1 if not set.",
			},
			gcli.IntFlag{
				Name:  "startIndex",
				Value: 0,
				Usage: "Index where deterministic key generation will start from. Assume 0 if not set.",
			},
			gcli.BoolFlag{
				Name:  "confirmAddress",
				Usage: "If requesting one address it will be sent only if user confirms operation by pressing device's button.",
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
			addressN := c.Int("addressN")
			startIndex := c.Int("startIndex")
			confirmAddress := c.Bool("confirmAddress")
			coinType, err := skyWallet.CoinTypeFromString(c.String("coinType"))
			if err != nil {
				log.Error(err)
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

			var pinEnc string
			msg, err := device.AddressGen(uint32(addressN), uint32(startIndex), confirmAddress, coinType)
			if err != nil {
				log.Error(err)
				return
			}

			for msg.Kind != uint16(messages.MessageType_MessageType_ResponseSkycoinAddress) && msg.Kind != uint16(messages.MessageType_MessageType_Failure) {
				if msg.Kind == uint16(messages.MessageType_MessageType_PinMatrixRequest) {
					fmt.Printf("PinMatrixRequest response: ")
					fmt.Scanln(&pinEnc)
					pinAckResponse, err := device.PinMatrixAck(pinEnc)
					if err != nil {
						log.Error(err)
						return
					}
					log.Infof("PinMatrixAck response: %s", pinAckResponse)
					continue
				}

				if msg.Kind == uint16(messages.MessageType_MessageType_PassphraseRequest) {
					var passphrase string
					fmt.Printf("Input passphrase: ")
					fmt.Scanln(&passphrase)
					passphraseAckResponse, err := device.PassphraseAck(passphrase)
					if err != nil {
						log.Error(err)
						return
					}
					log.Infof("PinMatrixAck response: %s", passphraseAckResponse)
					continue
				}

				if msg.Kind == uint16(messages.MessageType_MessageType_ButtonRequest) {
					msg, err = device.ButtonAck()
					if err != nil {
						log.Error(err)
						return
					}
					continue
				}
			}

			if msg.Kind == uint16(messages.MessageType_MessageType_ResponseSkycoinAddress) {
				addresses, err := skyWallet.DecodeResponseSkycoinAddress(msg)
				if err != nil {
					log.Error(err)
					return
				}
				fmt.Println(addresses)
			} else {
				failMsg, err := skyWallet.DecodeFailMsg(msg)
				if err != nil {
					log.Error(err)
					return
				}
				fmt.Println("Failed with code: ", failMsg)
				return
			}
		},
	}
}
