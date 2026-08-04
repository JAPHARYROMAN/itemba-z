import 'dart:async';
import 'dart:convert';
import 'dart:io';

class ApiRequest {
  const ApiRequest({
    required this.method,
    required this.uri,
    required this.headers,
    this.body,
  });

  final String method;
  final Uri uri;
  final Map<String, String> headers;
  final String? body;
}

class ApiResponse {
  const ApiResponse({
    required this.statusCode,
    required this.headers,
    required this.body,
  });

  final int statusCode;
  final Map<String, String> headers;
  final String body;
}

abstract interface class ApiTransport {
  Future<ApiResponse> send(ApiRequest request);
  Future<void> close();
}

class ApiTransportException implements Exception {
  const ApiTransportException();

  @override
  String toString() => 'The API could not be reached.';
}

class DartIoApiTransport implements ApiTransport {
  DartIoApiTransport({HttpClient? client}) : _client = client ?? HttpClient() {
    _client.connectionTimeout = const Duration(seconds: 15);
  }

  final HttpClient _client;

  @override
  Future<ApiResponse> send(ApiRequest request) async {
    try {
      final outgoing = await _client
          .openUrl(request.method, request.uri)
          .timeout(const Duration(seconds: 20));
      outgoing.followRedirects = false;
      request.headers.forEach(outgoing.headers.set);
      if (request.body case final body?) {
        outgoing.add(utf8.encode(body));
      }
      final incoming = await outgoing.close().timeout(
        const Duration(seconds: 30),
      );
      final responseBody = await incoming
          .transform(utf8.decoder)
          .join()
          .timeout(const Duration(seconds: 30));
      final headers = <String, String>{};
      incoming.headers.forEach((name, values) {
        headers[name.toLowerCase()] = values.join(',');
      });
      return ApiResponse(
        statusCode: incoming.statusCode,
        headers: headers,
        body: responseBody,
      );
    } on TimeoutException {
      throw const ApiTransportException();
    } on SocketException {
      throw const ApiTransportException();
    } on HandshakeException {
      throw const ApiTransportException();
    } on HttpException {
      throw const ApiTransportException();
    }
  }

  @override
  Future<void> close() async {
    _client.close(force: true);
  }
}

class InMemoryApiTransport implements ApiTransport {
  InMemoryApiTransport(this.handler);

  final FutureOr<ApiResponse> Function(ApiRequest request) handler;
  final List<ApiRequest> requests = [];

  @override
  Future<ApiResponse> send(ApiRequest request) async {
    requests.add(request);
    return handler(request);
  }

  @override
  Future<void> close() async {}
}
