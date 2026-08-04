import '../domain/models.dart';
import 'itemba_api_client.dart';
import 'sync_service.dart';

typedef SyncDelay = Future<void> Function(Duration duration);

class HttpAuthoritativeSyncGateway implements AuthoritativeSyncGateway {
  HttpAuthoritativeSyncGateway({
    required this.api,
    this.maximumAttempts = 3,
    this.initialBackoff = const Duration(milliseconds: 250),
    SyncDelay? delay,
  }) : _delay = delay ?? Future<void>.delayed;

  final ItembaApiClient api;
  final int maximumAttempts;
  final Duration initialBackoff;
  final SyncDelay _delay;

  @override
  Future<SyncResult> submit(SyncCommand command) async {
    if (maximumAttempts < 1) {
      throw ArgumentError.value(maximumAttempts, 'maximumAttempts');
    }
    ApiException? lastFailure;
    for (var offset = 0; offset < maximumAttempts; offset += 1) {
      try {
        return await api.syncSale(command, attempt: command.syncAttemptNumber);
      } on ApiException catch (error) {
        lastFailure = error;
        if (!error.isRetryable || offset == maximumAttempts - 1) {
          throw _syncFailure(error);
        }
        await _delay(initialBackoff * (1 << offset));
      }
    }
    throw _syncFailure(lastFailure!);
  }

  SyncFailure _syncFailure(ApiException error) => SyncFailure(
    kind: switch (error.kind) {
      ApiFailureKind.retryable => SyncFailureKind.retryable,
      ApiFailureKind.invalidResponse => SyncFailureKind.ambiguous,
      ApiFailureKind.authentication => SyncFailureKind.authentication,
      ApiFailureKind.suspended => SyncFailureKind.suspended,
      ApiFailureKind.terminal => SyncFailureKind.terminal,
    },
    message: error.message,
    code: error.code,
  );
}
