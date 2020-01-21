package utils

import (
	"context"
	"github.com/InVisionApp/go-health/v2"
	"github.com/jinzhu/gorm"
	"github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
	otgorm "github.com/smacker/opentracing-gorm"
	"google.golang.org/grpc"
)

// Env Context object supplied around the applications lifetime
type Env struct {
	wDb *gorm.DB
	rDb *gorm.DB

	Queue           stan.Conn

	Logger *logrus.Entry
	Health *health.Health

	profileServiceConn *grpc.ClientConn
}

func (env *Env) SetWriteDb(db *gorm.DB) {
	env.wDb = db
}

func (env *Env) SetReadDb(db *gorm.DB) {
	env.rDb = db
}

func (env *Env) GeWtDb(ctx context.Context) *gorm.DB {
	return otgorm.SetSpanToGorm(ctx, env.wDb)
}

func (env *Env) GetRDb(ctx context.Context) *gorm.DB {
	return otgorm.SetSpanToGorm(ctx, env.rDb)
}

// ConfigureProfileService creates required connection to the profile service
func (env *Env) GetProfileServiceConn() *grpc.ClientConn {

	if env.profileServiceConn != nil {
		return env.profileServiceConn
	}

	// Create a new interceptor
	jwt := &JWTInterceptor{
		// Set up all the members here
	}

	dialOption := grpc.WithInsecure()

	//
	//pool, err := x509.SystemCertPool()
	//if err != nil {
	//	env.Logger.Errorf("Could not get system certificates: %v", err)
	//	return nil
	//}
	//creds := credentials.NewClientTLSFromCert(pool, "")
	//dialOption = grpc.WithTransportCredentials(creds)
	//

	profileServiceUri := GetEnv(EnvProfileServiceUri, "")
	profileServiceConnection, err := grpc.Dial(
		profileServiceUri,
		dialOption,
		grpc.WithUnaryInterceptor(jwt.UnaryClientInterceptor))
	if err != nil {
		env.Logger.Errorf("Could not configure profile service connection: %v", err)
		return nil
	}

	env.profileServiceConn = profileServiceConnection
	return env.profileServiceConn

}
