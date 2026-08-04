import 'dart:async';

import 'package:flutter/material.dart';

import 'app.dart';
import 'application/mobile_runtime.dart';
import 'core/theme.dart';
import 'data/itemba_api_client.dart';
import 'domain/connection_models.dart';

class LiveMobileApp extends StatefulWidget {
  const LiveMobileApp({super.key, required this.runtime});

  final MobileRuntimeController runtime;

  @override
  State<LiveMobileApp> createState() => _LiveMobileAppState();
}

class _LiveMobileAppState extends State<LiveMobileApp> {
  @override
  void initState() {
    super.initState();
    unawaited(widget.runtime.initialize());
  }

  @override
  void dispose() {
    unawaited(widget.runtime.close());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: widget.runtime,
    builder: (context, _) {
      final controller = widget.runtime.salesController;
      if (controller != null &&
          const {
            LiveConnectionState.ready,
            LiveConnectionState.offlineCache,
          }.contains(widget.runtime.state)) {
        return ItembaSalesApp(controller: controller);
      }
      return MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: AppTheme.light,
        home:
            widget.runtime.state == LiveConnectionState.refreshing
                ? const _LoadingPage()
                : EnrollmentPage(runtime: widget.runtime),
      );
    },
  );
}

class _LoadingPage extends StatelessWidget {
  const _LoadingPage();

  @override
  Widget build(BuildContext context) => const Scaffold(
    body: SafeArea(
      child: Center(
        child: Padding(
          padding: EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircularProgressIndicator(),
              SizedBox(height: 20),
              Text(
                'Connecting securely…\nInaunganisha kwa usalama…',
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

class EnrollmentPage extends StatefulWidget {
  const EnrollmentPage({super.key, required this.runtime});

  final MobileRuntimeController runtime;

  @override
  State<EnrollmentPage> createState() => _EnrollmentPageState();
}

class _EnrollmentPageState extends State<EnrollmentPage> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _baseUrl;
  late final TextEditingController _deviceName;
  final _token = TextEditingController();
  late final TextEditingController _actorId;
  late final TextEditingController _tenantId;
  late final TextEditingController _companyId;
  late final TextEditingController _branchId;
  late final TextEditingController _warehouseId;
  MobileIdentityMode _mode = MobileIdentityMode.bearer;
  bool _swahili = false;

  @override
  void initState() {
    super.initState();
    final connection = widget.runtime.connection;
    final development = connection?.developmentIdentity;
    _baseUrl = TextEditingController(
      text: connection?.baseUrl.toString() ?? 'https://',
    );
    _deviceName = TextEditingController(
      text: widget.runtime.enrollment?.deviceName ?? '',
    );
    _actorId = TextEditingController(text: development?.actorId ?? '');
    _tenantId = TextEditingController(text: development?.tenantId ?? '');
    _companyId = TextEditingController(text: development?.companyId ?? '');
    _branchId = TextEditingController(text: development?.branchId ?? '');
    _warehouseId = TextEditingController(text: development?.warehouseId ?? '');
    if (allowDevelopmentIdentity &&
        connection?.identityMode == MobileIdentityMode.developmentHeaders) {
      _mode = MobileIdentityMode.developmentHeaders;
    }
  }

  @override
  void dispose() {
    _baseUrl.dispose();
    _deviceName.dispose();
    _token.dispose();
    _actorId.dispose();
    _tenantId.dispose();
    _companyId.dispose();
    _branchId.dispose();
    _warehouseId.dispose();
    super.dispose();
  }

  String _text(String english, String swahili) => _swahili ? swahili : english;

  @override
  Widget build(BuildContext context) {
    final busy = widget.runtime.busy;
    return Scaffold(
      appBar: AppBar(
        title: const Text('ITEMBA-Z Sales'),
        actions: [
          TextButton.icon(
            onPressed: busy ? null : () => setState(() => _swahili = !_swahili),
            icon: const Icon(Icons.translate),
            label: Text(_swahili ? 'English' : 'Kiswahili'),
          ),
        ],
      ),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 560),
              child: Card(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Form(
                    key: _formKey,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        const Icon(Icons.phonelink_lock, size: 52),
                        const SizedBox(height: 14),
                        Text(
                          _text('Enroll this device', 'Sajili kifaa hiki'),
                          style: Theme.of(context).textTheme.headlineSmall
                              ?.copyWith(fontWeight: FontWeight.w900),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 8),
                        Text(
                          _text(
                            'Use the secure API address and credential supplied by your administrator.',
                            'Tumia anwani salama ya API na kitambulisho ulichopewa na msimamizi.',
                          ),
                          textAlign: TextAlign.center,
                        ),
                        if (widget.runtime.errorMessage != null) ...[
                          const SizedBox(height: 16),
                          _ErrorBanner(
                            message: _text(
                              widget.runtime.errorMessage!,
                              'Muunganisho haujakamilika. Hakiki taarifa na ujaribu tena.',
                            ),
                          ),
                        ],
                        const SizedBox(height: 22),
                        TextFormField(
                          controller: _baseUrl,
                          enabled: !busy,
                          keyboardType: TextInputType.url,
                          autocorrect: false,
                          decoration: InputDecoration(
                            labelText: _text(
                              'Secure API URL',
                              'Anwani salama ya API',
                            ),
                            prefixIcon: const Icon(Icons.link),
                          ),
                          validator:
                              (value) =>
                                  Uri.tryParse(
                                            value?.trim() ?? '',
                                          )?.hasAuthority ==
                                          true
                                      ? null
                                      : _text(
                                        'Enter a valid URL.',
                                        'Weka anwani sahihi.',
                                      ),
                        ),
                        const SizedBox(height: 14),
                        TextFormField(
                          controller: _deviceName,
                          enabled: !busy,
                          decoration: InputDecoration(
                            labelText: _text('Device name', 'Jina la kifaa'),
                            prefixIcon: const Icon(Icons.phone_android),
                          ),
                          validator:
                              (value) =>
                                  (value?.trim().isNotEmpty ?? false)
                                      ? null
                                      : _text(
                                        'Device name is required.',
                                        'Jina la kifaa linahitajika.',
                                      ),
                        ),
                        if (allowDevelopmentIdentity) ...[
                          const SizedBox(height: 14),
                          DropdownButtonFormField<MobileIdentityMode>(
                            value: _mode,
                            decoration: InputDecoration(
                              labelText: _text(
                                'Identity mode',
                                'Aina ya utambulisho',
                              ),
                              prefixIcon: const Icon(Icons.badge_outlined),
                            ),
                            items: [
                              DropdownMenuItem(
                                value: MobileIdentityMode.bearer,
                                child: Text(
                                  _text(
                                    'Bearer credential',
                                    'Kitambulisho cha Bearer',
                                  ),
                                ),
                              ),
                              DropdownMenuItem(
                                value: MobileIdentityMode.developmentHeaders,
                                child: Text(
                                  _text(
                                    'Local development only',
                                    'Maendeleo ya ndani tu',
                                  ),
                                ),
                              ),
                            ],
                            onChanged:
                                busy
                                    ? null
                                    : (value) => setState(() => _mode = value!),
                          ),
                        ],
                        const SizedBox(height: 14),
                        if (_mode == MobileIdentityMode.bearer)
                          TextFormField(
                            controller: _token,
                            enabled: !busy,
                            obscureText: true,
                            enableSuggestions: false,
                            autocorrect: false,
                            decoration: InputDecoration(
                              labelText: _text(
                                'Bearer credential',
                                'Kitambulisho cha Bearer',
                              ),
                              prefixIcon: const Icon(Icons.key_outlined),
                            ),
                            validator:
                                (value) =>
                                    (value?.trim().isNotEmpty ?? false)
                                        ? null
                                        : _text(
                                          'Credential is required.',
                                          'Kitambulisho kinahitajika.',
                                        ),
                          )
                        else ...[
                          _UuidField(controller: _actorId, label: 'Actor ID'),
                          _UuidField(controller: _tenantId, label: 'Tenant ID'),
                          _UuidField(
                            controller: _companyId,
                            label: 'Company ID',
                          ),
                          _UuidField(controller: _branchId, label: 'Branch ID'),
                          _UuidField(
                            controller: _warehouseId,
                            label: 'Warehouse ID',
                          ),
                        ],
                        const SizedBox(height: 22),
                        FilledButton.icon(
                          onPressed: busy ? null : _submit,
                          icon:
                              busy
                                  ? const SizedBox.square(
                                    dimension: 18,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                  : const Icon(Icons.verified_user_outlined),
                          label: Text(
                            busy
                                ? _text('Enrolling…', 'Inasajili…')
                                : _text(
                                  'Enroll and connect',
                                  'Sajili na uunganishe',
                                ),
                          ),
                        ),
                        const SizedBox(height: 10),
                        Text(
                          _text(
                            'Release builds accept HTTPS and bearer credentials only.',
                            'Matoleo rasmi yanakubali HTTPS na kitambulisho cha Bearer pekee.',
                          ),
                          style: Theme.of(context).textTheme.bodySmall,
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final development =
        _mode == MobileIdentityMode.developmentHeaders
            ? DevelopmentIdentity(
              actorId: _actorId.text.trim(),
              tenantId: _tenantId.text.trim(),
              companyId: _companyId.text.trim(),
              branchId: _branchId.text.trim(),
              warehouseId: _warehouseId.text.trim(),
            )
            : null;
    await widget.runtime.enroll(
      baseUrl: Uri.parse(_baseUrl.text.trim()),
      deviceName: _deviceName.text.trim(),
      identityMode: _mode,
      bearerToken: _token.text,
      developmentIdentity: development,
    );
  }
}

class _UuidField extends StatelessWidget {
  const _UuidField({required this.controller, required this.label});

  final TextEditingController controller;
  final String label;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 12),
    child: TextFormField(
      controller: controller,
      autocorrect: false,
      decoration: InputDecoration(labelText: label),
      validator:
          (value) =>
              RegExp(
                    r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$',
                  ).hasMatch(value?.trim() ?? '')
                  ? null
                  : 'A UUID is required.',
    ),
  );
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(12),
    decoration: BoxDecoration(
      color: const Color(0xFFFFE8E8),
      borderRadius: BorderRadius.circular(12),
    ),
    child: Row(
      children: [
        const Icon(Icons.error_outline, color: Color(0xFFB42318)),
        const SizedBox(width: 10),
        Expanded(child: Text(message)),
      ],
    ),
  );
}
