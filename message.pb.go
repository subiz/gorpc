// Code generated by protoc-gen-go. DO NOT EDIT.
// source: message.proto

package gorpc

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type StatusResponse struct {
	Paths                []string `protobuf:"bytes,3,rep,name=paths,proto3" json:"paths,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StatusResponse) Reset()         { *m = StatusResponse{} }
func (m *StatusResponse) String() string { return proto.CompactTextString(m) }
func (*StatusResponse) ProtoMessage()    {}
func (*StatusResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_68a9f11280edc22d, []int{0}
}
func (m *StatusResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StatusResponse.Unmarshal(m, b)
}
func (m *StatusResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StatusResponse.Marshal(b, m, deterministic)
}
func (dst *StatusResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StatusResponse.Merge(dst, src)
}
func (m *StatusResponse) XXX_Size() int {
	return xxx_messageInfo_StatusResponse.Size(m)
}
func (m *StatusResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_StatusResponse.DiscardUnknown(m)
}

var xxx_messageInfo_StatusResponse proto.InternalMessageInfo

func (m *StatusResponse) GetPaths() []string {
	if m != nil {
		return m.Paths
	}
	return nil
}

type Request struct {
	Version              string            `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Id                   uint64            `protobuf:"varint,3,opt,name=id,proto3" json:"id,omitempty"`
	Uri                  []byte            `protobuf:"bytes,4,opt,name=uri,proto3" json:"uri,omitempty"`
	Method               string            `protobuf:"bytes,5,opt,name=method,proto3" json:"method,omitempty"`
	Body                 []byte            `protobuf:"bytes,6,opt,name=body,proto3" json:"body,omitempty"`
	ContentType          string            `protobuf:"bytes,7,opt,name=content_type,json=contentType,proto3" json:"content_type,omitempty"`
	UserAgent            []byte            `protobuf:"bytes,8,opt,name=user_agent,json=userAgent,proto3" json:"user_agent,omitempty"`
	Cookies              map[string][]byte `protobuf:"bytes,9,rep,name=cookies,proto3" json:"cookies,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Headers              map[string][]byte `protobuf:"bytes,10,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	RemoteAddr           string            `protobuf:"bytes,11,opt,name=remote_addr,json=remoteAddr,proto3" json:"remote_addr,omitempty"`
	Referer              string            `protobuf:"bytes,12,opt,name=referer,proto3" json:"referer,omitempty"`
	Received             int64             `protobuf:"varint,15,opt,name=received,proto3" json:"received,omitempty"`
	Path                 string            `protobuf:"bytes,16,opt,name=path,proto3" json:"path,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *Request) Reset()         { *m = Request{} }
func (m *Request) String() string { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()    {}
func (*Request) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_68a9f11280edc22d, []int{1}
}
func (m *Request) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Request.Unmarshal(m, b)
}
func (m *Request) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Request.Marshal(b, m, deterministic)
}
func (dst *Request) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Request.Merge(dst, src)
}
func (m *Request) XXX_Size() int {
	return xxx_messageInfo_Request.Size(m)
}
func (m *Request) XXX_DiscardUnknown() {
	xxx_messageInfo_Request.DiscardUnknown(m)
}

var xxx_messageInfo_Request proto.InternalMessageInfo

func (m *Request) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *Request) GetId() uint64 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *Request) GetUri() []byte {
	if m != nil {
		return m.Uri
	}
	return nil
}

func (m *Request) GetMethod() string {
	if m != nil {
		return m.Method
	}
	return ""
}

func (m *Request) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *Request) GetContentType() string {
	if m != nil {
		return m.ContentType
	}
	return ""
}

func (m *Request) GetUserAgent() []byte {
	if m != nil {
		return m.UserAgent
	}
	return nil
}

func (m *Request) GetCookies() map[string][]byte {
	if m != nil {
		return m.Cookies
	}
	return nil
}

func (m *Request) GetHeaders() map[string][]byte {
	if m != nil {
		return m.Headers
	}
	return nil
}

func (m *Request) GetRemoteAddr() string {
	if m != nil {
		return m.RemoteAddr
	}
	return ""
}

func (m *Request) GetReferer() string {
	if m != nil {
		return m.Referer
	}
	return ""
}

func (m *Request) GetReceived() int64 {
	if m != nil {
		return m.Received
	}
	return 0
}

func (m *Request) GetPath() string {
	if m != nil {
		return m.Path
	}
	return ""
}

type Response struct {
	Version              string            `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Id                   uint64            `protobuf:"varint,3,opt,name=id,proto3" json:"id,omitempty"`
	StatusCode           int32             `protobuf:"varint,4,opt,name=status_code,json=statusCode,proto3" json:"status_code,omitempty"`
	Body                 []byte            `protobuf:"bytes,5,opt,name=body,proto3" json:"body,omitempty"`
	Error                string            `protobuf:"bytes,9,opt,name=error,proto3" json:"error,omitempty"`
	Headers              map[string][]byte `protobuf:"bytes,10,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *Response) Reset()         { *m = Response{} }
func (m *Response) String() string { return proto.CompactTextString(m) }
func (*Response) ProtoMessage()    {}
func (*Response) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_68a9f11280edc22d, []int{2}
}
func (m *Response) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Response.Unmarshal(m, b)
}
func (m *Response) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Response.Marshal(b, m, deterministic)
}
func (dst *Response) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Response.Merge(dst, src)
}
func (m *Response) XXX_Size() int {
	return xxx_messageInfo_Response.Size(m)
}
func (m *Response) XXX_DiscardUnknown() {
	xxx_messageInfo_Response.DiscardUnknown(m)
}

var xxx_messageInfo_Response proto.InternalMessageInfo

func (m *Response) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *Response) GetId() uint64 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *Response) GetStatusCode() int32 {
	if m != nil {
		return m.StatusCode
	}
	return 0
}

func (m *Response) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *Response) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func (m *Response) GetHeaders() map[string][]byte {
	if m != nil {
		return m.Headers
	}
	return nil
}

func init() {
	proto.RegisterType((*StatusResponse)(nil), "gorpc.StatusResponse")
	proto.RegisterType((*Request)(nil), "gorpc.Request")
	proto.RegisterMapType((map[string][]byte)(nil), "gorpc.Request.CookiesEntry")
	proto.RegisterMapType((map[string][]byte)(nil), "gorpc.Request.HeadersEntry")
	proto.RegisterType((*Response)(nil), "gorpc.Response")
	proto.RegisterMapType((map[string][]byte)(nil), "gorpc.Response.HeadersEntry")
}

func init() { proto.RegisterFile("message.proto", fileDescriptor_message_68a9f11280edc22d) }

var fileDescriptor_message_68a9f11280edc22d = []byte{
	// 416 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xa4, 0x53, 0xcf, 0xcb, 0xd3, 0x40,
	0x10, 0x25, 0x4d, 0xd2, 0xb4, 0x93, 0xf8, 0xf9, 0xb1, 0x88, 0x2c, 0x9f, 0x8a, 0xb5, 0x07, 0xe9,
	0x29, 0x07, 0x45, 0x91, 0xde, 0x4a, 0x11, 0x3c, 0xaf, 0xde, 0x43, 0x9a, 0x1d, 0xdb, 0x50, 0x9b,
	0x8d, 0xbb, 0x9b, 0x42, 0xee, 0xfe, 0xc7, 0xfe, 0x03, 0xee, 0x8f, 0x34, 0x14, 0xe9, 0x41, 0xf9,
	0x6e, 0xf3, 0xde, 0xce, 0x0b, 0xf3, 0xe6, 0x4d, 0xe0, 0xc9, 0x09, 0x95, 0x2a, 0xf7, 0x98, 0xb7,
	0x52, 0x68, 0x41, 0xe2, 0xbd, 0x90, 0x6d, 0xb5, 0x7c, 0x0b, 0x77, 0x5f, 0x75, 0xa9, 0x3b, 0xc5,
	0x50, 0xb5, 0xa2, 0x51, 0x48, 0x9e, 0x41, 0xdc, 0x96, 0xfa, 0xa0, 0x68, 0xb8, 0x08, 0x57, 0x73,
	0xe6, 0xc1, 0xf2, 0x57, 0x04, 0x09, 0xc3, 0x9f, 0x1d, 0x2a, 0x4d, 0x28, 0x24, 0x67, 0x94, 0xaa,
	0x16, 0x0d, 0x9d, 0x2c, 0x02, 0xd3, 0x73, 0x81, 0xe4, 0x0e, 0x26, 0x35, 0x37, 0xc2, 0x60, 0x15,
	0x31, 0x53, 0x91, 0x7b, 0x08, 0x3b, 0x59, 0xd3, 0xc8, 0x10, 0x19, 0xb3, 0x25, 0x79, 0x0e, 0xd3,
	0x13, 0xea, 0x83, 0xe0, 0x34, 0x76, 0xd2, 0x01, 0x11, 0x02, 0xd1, 0x4e, 0xf0, 0x9e, 0x4e, 0x5d,
	0xab, 0xab, 0xc9, 0x1b, 0xc8, 0x2a, 0xd1, 0x68, 0x6c, 0x74, 0xa1, 0xfb, 0x16, 0x69, 0xe2, 0x14,
	0xe9, 0xc0, 0x7d, 0x33, 0x14, 0x79, 0x05, 0xd0, 0x29, 0x94, 0x85, 0xf1, 0xd5, 0x68, 0x3a, 0x73,
	0xe2, 0xb9, 0x65, 0x36, 0x96, 0x20, 0x1f, 0x20, 0xa9, 0x84, 0x38, 0xd6, 0xa8, 0xe8, 0xdc, 0xb8,
	0x49, 0xdf, 0xbd, 0xc8, 0x9d, 0xed, 0x7c, 0xb0, 0x92, 0x6f, 0xfd, 0xeb, 0xe7, 0x46, 0xcb, 0x9e,
	0x5d, 0x7a, 0xad, 0xec, 0x80, 0x25, 0x37, 0xa6, 0x28, 0xdc, 0x94, 0x7d, 0xf1, 0xaf, 0x83, 0x6c,
	0xe8, 0x25, 0xaf, 0x21, 0x95, 0x78, 0x12, 0x1a, 0x8b, 0x92, 0x73, 0x49, 0x53, 0x37, 0x2e, 0x78,
	0x6a, 0x63, 0x18, 0xbb, 0x38, 0x89, 0xdf, 0x51, 0xa2, 0xa4, 0x99, 0x5f, 0xdc, 0x00, 0xc9, 0x03,
	0xcc, 0x24, 0x56, 0x58, 0x9f, 0x91, 0xd3, 0xa7, 0xe6, 0x29, 0x64, 0x23, 0xb6, 0xab, 0xb1, 0x19,
	0xd0, 0x7b, 0x27, 0x71, 0xf5, 0xc3, 0x1a, 0xb2, 0xeb, 0xd1, 0xed, 0xa2, 0x8f, 0xd8, 0xd3, 0xc0,
	0xb5, 0xd8, 0xd2, 0xc6, 0x78, 0x2e, 0x7f, 0x74, 0xe8, 0x22, 0xca, 0x98, 0x07, 0xeb, 0xc9, 0xa7,
	0xc0, 0x6a, 0xaf, 0xe7, 0xff, 0x1f, 0xed, 0xf2, 0x77, 0x00, 0xb3, 0xf1, 0x52, 0xfe, 0xfd, 0x0e,
	0xcc, 0x66, 0x94, 0xbb, 0xb2, 0xa2, 0x12, 0x1c, 0xdd, 0x3d, 0xc4, 0x0c, 0x3c, 0xb5, 0x35, 0xcc,
	0x18, 0x7f, 0x7c, 0x15, 0xbf, 0x99, 0x02, 0xa5, 0x14, 0xd2, 0x44, 0x67, 0x3f, 0xee, 0x01, 0xf9,
	0xf8, 0x77, 0x36, 0x2f, 0xc7, 0x6c, 0xfc, 0x58, 0xb7, 0xc3, 0x79, 0x8c, 0xeb, 0xdd, 0xd4, 0xfd,
	0x32, 0xef, 0xff, 0x04, 0x00, 0x00, 0xff, 0xff, 0xa1, 0x83, 0x37, 0x96, 0x43, 0x03, 0x00, 0x00,
}
