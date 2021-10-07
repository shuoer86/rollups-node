#![warn(unused_extern_crates)]
use state_server_grpc::{serve_delegate_manager, wait_for_signal};

use offchain::config::{DescartesCLIConfig, DescartesConfig};
use structopt::StructOpt;
use tokio::sync::oneshot;

#[derive(StructOpt)]
struct ServerConfig {
    #[structopt(flatten)]
    descartes_cli_config: DescartesCLIConfig,
    #[structopt(flatten)]
    server_type: DelegateServerType,
}

#[derive(StructOpt)]
enum DelegateServerType {
    Input,
    Output,
    Rollups,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let (shutdown_tx, shutdown_rx) = oneshot::channel();

    let _ = tokio::spawn(wait_for_signal(shutdown_tx));

    let server_config = ServerConfig::from_args();
    let descartes_config =
        DescartesConfig::initialize(server_config.descartes_cli_config)
            .unwrap();

    match server_config.server_type {
        DelegateServerType::Input => {
            let input_fold = delegate_server::instantiate_input_fold_delegate(&descartes_config);

            serve_delegate_manager(
                "[::1]:50051",
                delegate_server::input_server::InputDelegateManager {
                    fold: input_fold,
                },
                shutdown_rx,
            )
            .await
        }
        DelegateServerType::Output => {
            let output_fold =
                delegate_server::instantiate_output_fold_delegate(&descartes_config);

            serve_delegate_manager(
                "[::1]:50051",
                delegate_server::output_server::OutputDelegateManager {
                    fold: output_fold,
                },
                shutdown_rx,
            )
            .await
        }
        DelegateServerType::Rollups => {
            let descartes_fold =
                delegate_server::instantiate_descartes_fold_delegate(&descartes_config);

            serve_delegate_manager(
                "[::1]:50051",
                delegate_server::rollups_server::RollupsDelegateManager {
                    fold: descartes_fold,
                },
                shutdown_rx,
            )
            .await
        }
    }
}
