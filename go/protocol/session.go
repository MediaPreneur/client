// Auto-generated by avdl-compiler v1.1.1 (https://github.com/keybase/node-avdl-compiler)
//   Input file: avdl/session.avdl

package keybase1

import (
	rpc "github.com/keybase/go-framed-msgpack-rpc"
	context "golang.org/x/net/context"
)

type Session struct {
	Uid             UID    `codec:"uid" json:"uid"`
	Username        string `codec:"username" json:"username"`
	Token           string `codec:"token" json:"token"`
	DeviceSubkeyKid KID    `codec:"deviceSubkeyKid" json:"deviceSubkeyKid"`
	DeviceSibkeyKid KID    `codec:"deviceSibkeyKid" json:"deviceSibkeyKid"`
}

type CurrentSessionArg struct {
	SessionID int `codec:"sessionID" json:"sessionID"`
}

type SessionInterface interface {
	CurrentSession(context.Context, int) (Session, error)
}

func SessionProtocol(i SessionInterface) rpc.Protocol {
	return rpc.Protocol{
		Name: "keybase.1.session",
		Methods: map[string]rpc.ServeHandlerDescription{
			"currentSession": {
				MakeArg: func() interface{} {
					ret := make([]CurrentSessionArg, 1)
					return &ret
				},
				Handler: func(ctx context.Context, args interface{}) (ret interface{}, err error) {
					typedArgs, ok := args.(*[]CurrentSessionArg)
					if !ok {
						err = rpc.NewTypeError((*[]CurrentSessionArg)(nil), args)
						return
					}
					ret, err = i.CurrentSession(ctx, (*typedArgs)[0].SessionID)
					return
				},
				MethodType: rpc.MethodCall,
			},
		},
	}
}

type SessionClient struct {
	Cli rpc.GenericClient
}

func (c SessionClient) CurrentSession(ctx context.Context, sessionID int) (res Session, err error) {
	__arg := CurrentSessionArg{SessionID: sessionID}
	err = c.Cli.Call(ctx, "keybase.1.session.currentSession", []interface{}{__arg}, &res)
	return
}
