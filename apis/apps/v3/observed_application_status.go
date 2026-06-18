package v3

import "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"

// ObservedApplicationStatus mirrors Controller application status (client.ApplicationInfo).
type ObservedApplicationStatus client.ApplicationInfo

// DeepCopyInto copies the receiver into out.
func (in *ObservedApplicationStatus) DeepCopyInto(out *ObservedApplicationStatus) {
	if in == nil {
		return
	}
	src := (*client.ApplicationInfo)(in)
	dst := (*client.ApplicationInfo)(out)
	*dst = *src
	if src.Microservices != nil {
		dst.Microservices = make([]client.MicroserviceInfo, len(src.Microservices))
		copy(dst.Microservices, src.Microservices)
	}
	if src.NatsConfig != nil {
		nc := *src.NatsConfig
		dst.NatsConfig = &nc
	}
}

// DeepCopy returns a deep copy of the receiver.
func (in *ObservedApplicationStatus) DeepCopy() *ObservedApplicationStatus {
	if in == nil {
		return nil
	}
	out := new(ObservedApplicationStatus)
	in.DeepCopyInto(out)
	return out
}
