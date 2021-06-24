package cli

import (
	"os"
	"runtime"
	"fmt"
	"strings"

	gcli "github.com/urfave/cli"

	messages "github.com/skycoin/hardware-wallet-protob/go"

	skyWallet "github.com/skycoin/hardware-wallet-go/src/skywallet"
)

func deterministicBuildCmd() gcli.Command {
  name := "deterministicBuild"
  return gcli.Command{
    Name:        name,
		Usage:       "Checks deterministic builds (for developer purposes only)",
		Description: "",

		Flags: []gcli.Flag{
			gcli.StringFlag{
				Name:  "mnemonic",
				Usage: "Mnemonic that will be stored in the device to generate addresses.",
			},
			gcli.StringFlag{
				Name:  "file_name",
				Usage: "Name of file to store results of a tool",
			},
			gcli.StringFlag{
				Name:  "file_action",
				Usage: "Two options to interact with file - overwrite (OVERWRITE) or append to existed (APPEND)",
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
			//grt coinType (only SKY)
			coinType, err := skyWallet.CoinTypeFromString(c.String("coinType"))

			file_name := c.String("file_name")

			if err != nil {
      	log.Error(err)
      	return
    	}

			//initialize HW
    	device := skyWallet.NewDevice(skyWallet.DeviceTypeFromString(c.String("deviceType")))
    		if device == nil {
      return
    	}

			defer device.Close()

			//set AUTO_PRESS_BUTTONS
			if os.Getenv("AUTO_PRESS_BUTTONS") == "1" && device.Driver.DeviceType() == skyWallet.DeviceTypeEmulator && runtime.GOOS == "linux" {
				err := device.SetAutoPressButton(true, skyWallet.ButtonRight)
				if err != nil {
					log.Error(err)
					return
				}
			}

			toolResult := result{Mnemonic: c.String("mnemonic")}

			start := time.Now()

    	switch coinType {
    		case skyWallet.SkycoinCoinType:

					//set mnemonic
					mnemonic := c.String("mnemonic")
					msg, err := device.SetMnemonic(mnemonic)
					if err != nil {
						log.Error(err)
						return
					}

					if msg.Kind == uint16(messages.MessageType_MessageType_ButtonRequest) {
						msg, err = device.ButtonAck()
						if err != nil {
							log.Error(err)
							return
						}
					}

					responseMsg, err := skyWallet.DecodeSuccessOrFailMsg(msg)
					if err != nil {
						log.Error(err)
						return
					}

					fmt.Println(responseMsg)

					//if device already with mnemonic - wipe and generate new
					if strings.Compare(responseMsg,
						"Device is already initialized. Use Wipe first.") == 0 {
						fmt.Println("Wiping...")

						msg, err := device.Wipe()
						if err != nil {
							log.Error(err)
							return
						}

						if msg.Kind == uint16(messages.MessageType_MessageType_ButtonRequest) {
							msg, err = device.ButtonAck()
							if err != nil {
								log.Error(err)
								return
							}
						}

						responseMsg, err := skyWallet.DecodeSuccessOrFailMsg(msg)
						if err != nil {
							log.Error(err)
							return
						}

						if len(responseMsg) > 0 {
							fmt.Println(responseMsg)
						} else {
							fmt.Println("Firmware was successfully wiped from the device")
						}

						msg, err = device.SetMnemonic(mnemonic)

					  if err != nil {
					    log.Error(err)
					  }

					  if msg.Kind == uint16(messages.MessageType_MessageType_ButtonRequest) {
					    // Send ButtonAck
					    msg, err = device.ButtonAck()
					    if err != nil {
					      log.Error(err)
					    }
					  }

					  responseMsg, err = skyWallet.DecodeSuccessOrFailMsg(msg)
					  if err != nil {
					    log.Error(err)
					  }

						fmt.Println(responseMsg)
					}

					//get keypair and address
					msg, err = device.AddressGen(1, 1, false, coinType)

					if err != nil {
						log.Error(err)
					}

					for msg.Kind != uint16(messages.MessageType_MessageType_ResponseSkycoinAddress) && msg.Kind != uint16(messages.MessageType_MessageType_Failure) {
						fmt.Println("Error")
					}
					if msg.Kind == uint16(messages.MessageType_MessageType_ResponseSkycoinAddress) {
						addresses, err := skyWallet.DecodeResponseSkycoinAddress(msg)
						if err != nil {
							log.Error(err)
						}
						fmt.Println(addresses)
						toolResult.Address = addresses[0]
					} else {
						failMsg, err := skyWallet.DecodeFailMsg(msg)
						if err != nil {
							log.Error(err)
						}
						fmt.Println("Failed with code: ", failMsg)
					}
				default:
					fmt.Println("Error")
    		}
				toolResult.Duration = time.Since(start).Nanoseconds()

				string_to_write := fmt.Sprintf("Mnemonic: %s, Duration: %d, Address: %s", toolResult.Mnemonic, toolResult.Duration, toolResult.Address)

				fmt.Println(string_to_write)

				file, _ := json.MarshalIndent(toolResult, "", " ")

				if strings.Compare(c.String("file_action"), "APPEND") == 0{

						f, err := os.OpenFile(file_name, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)

						if err != nil{
							log.Error(err)
						}

						defer f.Close()

						if _, err = f.WriteString(string(file)); err != nil {
							log.Error(err)
						}

				}else{
						f, err := os.OpenFile(file_name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)

						if err != nil{
							log.Error(err)
						}

						defer f.Close()

						if _, err = f.WriteString(string(file)); err != nil {
							log.Error(err)
						}
				}

  	},
	}
}
