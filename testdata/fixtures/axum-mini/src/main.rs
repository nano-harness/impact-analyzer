use axum::{routing::{get, post, put}, Router};

async fn list() {}
async fn create() {}
async fn detail() {}
async fn update() {}
async fn health() {}

fn app() -> Router {
    let api = Router::new()
        .route("/items", get(list).post(create))
        .route("/items/:id", get(detail).put(update));
    let v1 = Router::new().nest("/v1", api);
    Router::new()
        .route("/health", get(health))
        .nest("/api", v1)
}
