import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';

part 'sales_remote_datasource.g.dart';

/// Retrofit client for the outbound-sales module. Starts with the catalog
/// product CRUD (US-004 backend, mounted under `/products`, admin + paid).
/// Later sales stories (funnels, leads) extend this same datasource.
@RestApi()
abstract class SalesRemoteDataSource {
  factory SalesRemoteDataSource(Dio dio) = _SalesRemoteDataSource;

  @GET('/products')
  Future<HttpResponse<dynamic>> listProducts();

  @POST('/products')
  Future<HttpResponse<dynamic>> createProduct(
    @Body() Map<String, dynamic> body,
  );

  @GET('/products/{id}')
  Future<HttpResponse<dynamic>> getProduct(@Path('id') String id);

  @PUT('/products/{id}')
  Future<HttpResponse<dynamic>> updateProduct(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/products/{id}')
  Future<HttpResponse<dynamic>> deleteProduct(@Path('id') String id);

  /// The brochure a product was read from: filename, page, and a short-lived
  /// URL. Fetched on demand — a signed URL in the list payload would be a
  /// hundred links expiring in a drawer nobody opened.
  @GET('/products/{id}/source')
  Future<HttpResponse<dynamic>> getProductSource(@Path('id') String id);

  /// The pictures printed on the page this product was read from, each with a
  /// short-lived signed URL. Fetched when the product is opened, not carried
  /// in the list: signed URLs expire, and a hundred of them in a drawer
  /// nobody opened is a hundred dead links.
  @GET('/products/{id}/images')
  Future<HttpResponse<dynamic>> getProductImages(@Path('id') String id);

  /// The account's "why ours" library. Read by anyone on the floor — a seller
  /// needs to know the argument.
  @GET('/differentiators')
  Future<HttpResponse<dynamic>> getDifferentiators();

  /// Replaces the whole list, in order. Admin-only server-side.
  @PUT('/differentiators')
  Future<HttpResponse<dynamic>> saveDifferentiators(
    @Body() Map<String, dynamic> body,
  );

  // --- Sales funnels (US-006 backend, mounted under `/funnels`) -------------

  @GET('/funnels')
  Future<HttpResponse<dynamic>> listFunnels();

  @POST('/funnels')
  Future<HttpResponse<dynamic>> createFunnel(
    @Body() Map<String, dynamic> body,
  );

  @GET('/funnels/{id}')
  Future<HttpResponse<dynamic>> getFunnel(@Path('id') String id);

  @PUT('/funnels/{id}')
  Future<HttpResponse<dynamic>> updateFunnel(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/funnels/{id}')
  Future<HttpResponse<dynamic>> deleteFunnel(@Path('id') String id);

  /// Fires a signed sample webhook event so the admin can validate the funnel's
  /// webhook config (US-010 button → US-017 backend endpoint). Returns the
  /// client's HTTP status + body wrapped by the API.
  @POST('/funnels/{id}/webhook/test')
  Future<HttpResponse<dynamic>> testFunnelWebhook(@Path('id') String id);

  // --- Leads & pipeline (US-015/US-020/US-023, mounted under `/leads`) ------

  /// Lists a funnel's leads, optionally filtered by pipeline status (US-015).
  @GET('/funnels/{id}/leads')
  Future<HttpResponse<dynamic>> listFunnelLeads(
    @Path('id') String funnelId,
    @Query('status') String? status,
  );

  /// Runs a Google Places prospecting search for a funnel and creates leads
  /// from the results (US-022).
  @POST('/funnels/{id}/prospecting')
  Future<HttpResponse<dynamic>> prospecting(
    @Path('id') String funnelId,
    @Body() Map<String, dynamic> body,
  );

  /// Runs the same search WITHOUT creating anything, so the person can
  /// choose (§2 D4). A bad keyword used to spend the month's lead allowance
  /// on a list nobody would have picked.
  @POST('/funnels/{id}/prospecting/preview')
  Future<HttpResponse<dynamic>> prospectingPreview(
    @Path('id') String funnelId,
    @Body() Map<String, dynamic> body,
  );

  /// Imports the chosen candidates as leads.
  @POST('/funnels/{id}/prospecting/import')
  Future<HttpResponse<dynamic>> prospectingImport(
    @Path('id') String funnelId,
    @Body() Map<String, dynamic> body,
  );

  /// Kicks off async decision-maker enrichment via Gemini Grounding (US-023).
  /// Returns 202; the lead moves enriching → enriched in the background.
  @POST('/leads/{id}/enrich')
  Future<HttpResponse<dynamic>> enrichLead(@Path('id') String id);

  /// Generates an AI-suggested approach draft and queues the lead for human
  /// approval (status=pending_approval). Sends nothing (US-020).
  @POST('/leads/{id}/draft')
  Future<HttpResponse<dynamic>> draftLead(@Path('id') String id);

  /// Sends the human-edited final approach text via the funnel's channel,
  /// enforcing plan limits + the opt-out block (US-020).
  @POST('/leads/{id}/approve-send')
  Future<HttpResponse<dynamic>> approveSendLead(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Moves a lead through the pipeline (e.g. discard → lost) (US-015).
  @PATCH('/leads/{id}/status')
  Future<HttpResponse<dynamic>> updateLeadStatus(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );
}

/// Sales-history CSV import ("vendas como fonte única", análise 2026-09-01).
///
/// Deliberately NOT Retrofit methods on [SalesRemoteDataSource]: the preview
/// is a multipart upload, and the proven shape for those in this codebase is
/// the customer import's plain-Dio `FormData` call — same two-call split,
/// same `file` field the Go handler reads (`sale_import_handler.go`).
class SalesImportDataSource {
  const SalesImportDataSource(this._dio);

  final Dio _dio;

  /// Uploads the file and returns what the server made of it
  /// (`POST /sales/import/preview`, multipart field `file`). Creates nothing.
  Future<Map<String, dynamic>> previewSalesImport({
    required Uint8List bytes,
    required String filename,
  }) async {
    final form = FormData.fromMap(<String, dynamic>{
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final res = await _dio.post<Map<String, dynamic>>(
      '/sales/import/preview',
      data: form,
    );
    return res.data ?? const <String, dynamic>{};
  }

  /// Records a sale per confirmed row (`POST /sales/import`).
  ///
  /// Sends back the ROWS the preview showed rather than the file: she
  /// approved what she saw. The server re-derives every row anyway.
  Future<Map<String, dynamic>> importSales(
    List<Map<String, dynamic>> rows,
  ) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/sales/import',
      data: <String, dynamic>{'rows': rows},
    );
    return res.data ?? const <String, dynamic>{};
  }
}
