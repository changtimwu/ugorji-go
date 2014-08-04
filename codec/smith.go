package codec

import (
	"fmt"
	"io"
	"net/rpc"
)

type smithSpecRpcCodec struct {
	rpcCodec
}

// /////////////// Spec RPC Codec ///////////////////
func (c *smithSpecRpcCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	// WriteRequest can write to both a Go service, and other services that do
	// not abide by the 1 argument rule of a Go service.
	// We discriminate based on if the body is a MsgpackSpecRpcMultiArgs
	var bodyArr []interface{}
	if m, ok := body.(MsgpackSpecRpcMultiArgs); ok {
		bodyArr = ([]interface{})(m)
	} else {
		bodyArr = []interface{}{body}
	}
	//r2 := []interface{}{ 0, uint32(r.Seq), r.ServiceMethod, bodyArr}
	r2 := []interface{}{r.ServiceMethod, bodyArr, map[string]int{"$": r.Seq}}

	return c.write(r2, nil, false, true)
}

func (c *smithSpecRpcCodec) WriteResponse(r *rpc.Response, body interface{}) error {
	var moe interface{}
	if r.Error != "" {
		moe = r.Error
	}
	if moe != nil && body != nil {
		body = nil
	}
	r2 := []interface{}{1, uint32(r.Seq), moe, body}
	return c.write(r2, nil, false, true)
}

func (c *smithSpecRpcCodec) ReadResponseHeader(r *rpc.Response) error {
	return c.parseCustomHeader(1, &r.Seq, &r.Error)
}

func (c *smithSpecRpcCodec) ReadRequestHeader(r *rpc.Request) error {
	return c.parseCustomHeader(0, &r.Seq, &r.ServiceMethod)
}

func (c *smithSpecRpcCodec) ReadRequestBody(body interface{}) error {
	if body == nil { // read and discard
		return c.read(nil)
	}
	bodyArr := []interface{}{body}
	return c.read(&bodyArr)
}

func (c *smithSpecRpcCodec) parseCustomHeader(expectTypeByte byte, msgid *uint64, methodOrError *string) (err error) {

	if c.cls {
		return io.EOF
	}

	// We read the response header by hand
	// so that the body can be decoded on its own from the stream at a later time.

	const fia byte = 0x94 //four item array descriptor value
	// Not sure why the panic of EOF is swallowed above.
	// if bs1 := c.dec.r.readn1(); bs1 != fia {
	// 	err = fmt.Errorf("Unexpected value for array descriptor: Expecting %v. Received %v", fia, bs1)
	// 	return
	// }
	var b byte
	b, err = c.br.ReadByte()
	if err != nil {
		return
	}
	if b != fia {
		err = fmt.Errorf("Unexpected value for array descriptor: Expecting %v. Received %v", fia, b)
		return
	}

	if err = c.read(&b); err != nil {
		return
	}
	if b != expectTypeByte {
		err = fmt.Errorf("Unexpected byte descriptor in header. Expecting %v. Received %v", expectTypeByte, b)
		return
	}
	if err = c.read(msgid); err != nil {
		return
	}
	if err = c.read(methodOrError); err != nil {
		return
	}
	return
}

//--------------------------------------------------

// msgpackSpecRpc is the implementation of Rpc that uses custom communication protocol
// as defined in the msgpack spec at https://github.com/msgpack-rpc/msgpack-rpc/blob/master/spec.md
type smithSpecRpc struct{}

// MsgpackSpecRpc implements Rpc using the communication protocol defined in
// the msgpack spec at https://github.com/msgpack-rpc/msgpack-rpc/blob/master/spec.md .
// Its methods (ServerCodec and ClientCodec) return values that implement RpcCodecBuffered.
var SmithSpecRpc smithSpecRpc

func (x smithSpecRpc) ServerCodec(conn io.ReadWriteCloser, h Handle) rpc.ServerCodec {
	return &smithSpecRpcCodec{newRPCCodec(conn, h)}
}

func (x smithSpecRpc) ClientCodec(conn io.ReadWriteCloser, h Handle) rpc.ClientCodec {
	return &smithSpecRpcCodec{newRPCCodec(conn, h)}
}
