package contract

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/aiohttp"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/apscheduler"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/axum"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/celery"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/clap"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/click"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/cobra"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/commander"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/cronlib"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/django"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/echo"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/express"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/fastapi"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/flask"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/gin"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/grpc"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/nestjs"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/nodecron"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/pygrpc"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/reactrouter"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/springmvc"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/springsched"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/tornado"
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/typer"
)

func init() {
	// Register all extractors
	// Go-based extractors
	cobraExt := cobra.NewCobraExtractor()
	// Enable SSA by default for cobra (can be disabled via config if needed)
	cobraExt.EnableSSA(true)
	Register(cobraExt)

	Register(&gin.GinExtractor{})
	Register(&echo.EchoExtractor{})
	Register(&grpc.GRPCExtractor{})
	Register(&cronlib.CronlibExtractor{})

	// JavaScript/TypeScript extractors
	Register(&express.ExpressExtractor{})
	Register(&commander.CommanderExtractor{})
	Register(&nestjs.NestJSExtractor{})
	Register(&reactrouter.ReactRouterExtractor{})
	Register(&nodecron.NodeCronExtractor{})

	// Java extractors
	Register(&springmvc.SpringMVCExtractor{})
	Register(&springsched.SpringSchedExtractor{})

	// Rust extractors
	Register(&axum.AxumExtractor{})
	Register(&clap.ClapExtractor{})

	// Python extractors
	Register(&fastapi.FastAPIExtractor{})
	Register(&flask.FlaskExtractor{})
	Register(&django.DjangoExtractor{})
	Register(&click.ClickExtractor{})
	Register(&typer.TyperExtractor{})
	Register(&celery.CeleryExtractor{})
	Register(&apscheduler.APSchedulerExtractor{})
	Register(&pygrpc.PyGRPCExtractor{})
	Register(&tornado.TornadoExtractor{})
	Register(&aiohttp.AioHTTPExtractor{})
}
